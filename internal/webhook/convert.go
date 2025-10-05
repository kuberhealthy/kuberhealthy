package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	log "github.com/sirupsen/logrus"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	legacyCleanupTimeout = 30 * time.Second
	legacyCleanupRetry   = 500 * time.Millisecond
)

type (
	checkCreatorFunc   func(context.Context, *khapi.KuberhealthyCheck) error
	legacyDeleterFunc  func(context.Context, string, string) error
	cleanupSchedulerFn func(string, string, legacyDeleterFunc)
)

var (
	createCheckFunc       checkCreatorFunc
	legacyDeleteFunc      legacyDeleterFunc
	scheduleLegacyCleanup cleanupSchedulerFn = defaultLegacyCleanup
)

// SetLegacyHandlers allows tests or the main program to override how legacy
// resources are persisted and cleaned up. It returns a restore function so
// callers can revert to the previous configuration.
func SetLegacyHandlers(create checkCreatorFunc, delete legacyDeleterFunc, scheduler cleanupSchedulerFn) func() {
	prevCreate := createCheckFunc
	prevDelete := legacyDeleteFunc
	prevScheduler := scheduleLegacyCleanup

	createCheckFunc = create
	legacyDeleteFunc = delete
	if scheduler != nil {
		scheduleLegacyCleanup = scheduler
	}

	return func() {
		createCheckFunc = prevCreate
		legacyDeleteFunc = prevDelete
		scheduleLegacyCleanup = prevScheduler
	}
}

// ConfigureClient wires a controller-runtime client into the legacy
// conversion path so converted resources can be created and legacy ones can be
// removed.
func ConfigureClient(cl client.Client) {
	if cl == nil {
		return
	}

	create := func(ctx context.Context, check *khapi.KuberhealthyCheck) error {
		if check == nil {
			return fmt.Errorf("nil check provided to creator")
		}
		copy := check.DeepCopy()
		resetMetadataForCreate(&copy.ObjectMeta)
		copy.SetGroupVersionKind(khapi.GroupVersion.WithKind("KuberhealthyCheck"))
		err := cl.Create(ctx, copy)
		if apierrors.IsAlreadyExists(err) {
			existing := &khapi.KuberhealthyCheck{}
			e := cl.Get(ctx, client.ObjectKeyFromObject(copy), existing)
			if e != nil {
				return e
			}
			existing.Labels = copy.Labels
			existing.Annotations = copy.Annotations
			existing.Spec = copy.Spec
			existing.SetGroupVersionKind(khapi.GroupVersion.WithKind("KuberhealthyCheck"))
			return cl.Update(ctx, existing)
		}
		return err
	}

	deleteFn := func(ctx context.Context, namespace, name string) error {
		if name == "" {
			return nil
		}
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "comcast.github.io", Version: "v1", Kind: "KuberhealthyCheck"})
		obj.SetNamespace(namespace)
		obj.SetName(name)
		err := cl.Delete(ctx, obj)
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	SetLegacyHandlers(create, deleteFn, nil)
}

// resetMetadataForCreate removes fields that block create/update calls when
// cloning a legacy resource into the modern API group.
func resetMetadataForCreate(meta *metav1.ObjectMeta) {
	if meta == nil {
		return
	}
	meta.UID = ""
	meta.ResourceVersion = ""
	meta.Generation = 0
	meta.CreationTimestamp = metav1.Time{}
	meta.ManagedFields = nil
	meta.SelfLink = ""
}

// defaultLegacyCleanup schedules background deletion attempts for the legacy
// resource until the API server confirms removal or the timeout expires.
func defaultLegacyCleanup(namespace, name string, deleter legacyDeleterFunc) {
	if deleter == nil || name == "" {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), legacyCleanupTimeout)
		defer cancel()

		ticker := time.NewTicker(legacyCleanupRetry)
		defer ticker.Stop()

		for {
			err := deleter(ctx, namespace, name)
			if err == nil {
				return
			}
			if apierrors.IsNotFound(err) {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					continue
				}
			}

			log.WithError(err).Warnf("legacy webhook cleanup retry for %s/%s", namespace, name)

			select {
			case <-ctx.Done():
				log.Warnf("legacy webhook cleanup timed out removing %s/%s", namespace, name)
				return
			case <-ticker.C:
			}
		}
	}()
}

// Convert handles AdmissionReview requests for legacy Kuberhealthy checks and
// returns a response that upgrades them to the v2 API.
func Convert(w http.ResponseWriter, r *http.Request) {
	// read the AdmissionReview payload supplied by the Kubernetes API server
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read body: %v", err), http.StatusBadRequest)
		return
	}

	// decode the AdmissionReview so the request can be examined and updated
	review := admissionv1.AdmissionReview{}
	err = json.Unmarshal(body, &review)
	if err != nil {
		http.Error(w, fmt.Sprintf("unmarshal review: %v", err), http.StatusBadRequest)
		return
	}

	// build a conversion response that upgrades the incoming resource when needed
	review.Response = convertReview(r.Context(), &review)
	if review.Request != nil {
		review.Response.UID = review.Request.UID
	}

	// encode the response for transmission back through the webhook
	respBytes, err := json.Marshal(review)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshal response: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(respBytes)
	if err != nil {
		log.Errorln("write response:", err)
	}
}

// convertReview creates an AdmissionResponse converting legacy checks to v2.
func convertReview(ctx context.Context, ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	if ar.Request == nil {
		log.WithFields(log.Fields{"reason": "nil request"}).Info("legacy webhook passthrough")
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	if ar.Request.Operation == admissionv1.Delete {
		log.WithFields(log.Fields{
			"operation": admissionv1.Delete,
			"resource":  ar.Request.Resource.Resource,
			"group":     ar.Request.Resource.Group,
			"version":   ar.Request.Resource.Version,
			"namespace": ar.Request.Namespace,
			"name":      ar.Request.Name,
		}).Info("legacy webhook bypassed delete operation")
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	// read the raw request and inspect the type information of the resource
	raw := ar.Request.Object.Raw
	meta := metav1.TypeMeta{}
	err := json.Unmarshal(raw, &meta)
	f := log.Fields{
		"operation": ar.Request.Operation,
		"resource":  ar.Request.Resource.Resource,
		"group":     ar.Request.Resource.Group,
		"version":   ar.Request.Resource.Version,
		"namespace": ar.Request.Namespace,
		"name":      ar.Request.Name,
	}

	if err != nil {
		log.WithError(err).WithFields(f).Error("legacy webhook failed to parse typemeta")
		return toError(fmt.Errorf("parse typemeta: %w", err))
	}

	if meta.APIVersion == "" && ar.Request.Resource.Group != "" && ar.Request.Resource.Version != "" {
		meta.APIVersion = fmt.Sprintf("%s/%s", ar.Request.Resource.Group, ar.Request.Resource.Version)
	}
	if meta.Kind == "" {
		meta.Kind = legacyKindFromResource(ar.Request.Resource.Resource)
	}

	f["apiVersion"] = meta.APIVersion
	f["kind"] = meta.Kind

	legacyGroup := meta.APIVersion == "comcast.github.io/v1" || ar.Request.Resource.Group == "comcast.github.io"
	if !legacyGroup {
		log.WithFields(f).Info("legacy webhook passthrough")
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	// attempt to convert the incoming legacy object into the modern representation
	check, legacyObj, warning, err := convertLegacy(raw, meta.Kind)
	if err != nil {
		log.WithError(err).WithFields(f).Error("legacy webhook conversion failed")
		return toError(err)
	}
	if check == nil {
		log.WithFields(f).Info("legacy webhook passthrough")
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	// persist the converted object into the modern API group when configured
	if createCheckFunc != nil {
		createTarget := check.DeepCopy()
		resetMetadataForCreate(&createTarget.ObjectMeta)
		err = createCheckFunc(ctx, createTarget)
		if err != nil {
			log.WithError(err).WithFields(f).Error("legacy webhook failed to create converted resource")
			return toError(fmt.Errorf("create kuberhealthy.github.io/v2 check: %w", err))
		}
		if legacyObj != nil {
			scheduleLegacyCleanup(legacyObj.Namespace, legacyObj.Name, legacyDeleteFunc)
		}
	}

	// marshal the converted object and create a JSON patch from the original payload
	newRaw, err := json.Marshal(check)
	if err != nil {
		return toError(fmt.Errorf("marshal v2: %w", err))
	}

	ops, err := jsonpatch.CreatePatch(raw, newRaw)
	if err != nil {
		return toError(fmt.Errorf("create patch: %w", err))
	}
	patchBytes, err := json.Marshal(ops)
	if err != nil {
		log.WithError(err).WithFields(f).Error("legacy webhook failed to marshal patch")
		return toError(fmt.Errorf("marshal patch: %w", err))
	}

	pt := admissionv1.PatchTypeJSONPatch
	f["convertedTo"] = "kuberhealthy.github.io/v2"
	log.WithFields(f).Info("legacy webhook converted resource")
	return &admissionv1.AdmissionResponse{
		Allowed:   true,
		Patch:     patchBytes,
		PatchType: &pt,
		Warnings:  []string{warning},
	}
}

// convertLegacy upgrades a legacy Kuberhealthy object into a modern v2 check when supported.
func convertLegacy(raw []byte, kind string) (*khapi.KuberhealthyCheck, *legacyCheck, string, error) {
	switch kind {
	case "KuberhealthyCheck":
		// decode the legacy object and rewrite the API version to the current value
		out := khapi.KuberhealthyCheck{}
		err := json.Unmarshal(raw, &out)
		if err != nil {
			return nil, nil, "", fmt.Errorf("parse object: %w", err)
		}
		out.APIVersion = "kuberhealthy.github.io/v2"

		// populate missing pod spec details when the legacy payload used the v1 layout
		legacy := legacyCheck{}
		err = json.Unmarshal(raw, &legacy)
		if err != nil {
			return nil, nil, "", fmt.Errorf("parse legacy object: %w", err)
		}
		upgradeLegacyPodSpec(&out.Spec.PodSpec, legacy.Spec)

		return &out, &legacy, "converted legacy comcast.github.io/v1 KuberhealthyCheck to kuberhealthy.github.io/v2", nil
	default:
		return nil, nil, "", nil
	}
}

// legacyKindFromResource maps legacy resource aliases to their canonical kind.
func legacyKindFromResource(resource string) string {
	switch resource {
	case "khc", "khcheck", "khchecks", "kuberhealthycheck", "kuberhealthychecks":
		return "KuberhealthyCheck"
	default:
		return ""
	}
}

// legacyCheck captures the legacy v1 layout so we can normalize it.
type legacyCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              legacyCheckSpec `json:"spec,omitempty"`
}

// legacyCheckSpec reflects the comcast.github.io/v1 spec structure.
type legacyCheckSpec struct {
	RunInterval      *metav1.Duration  `json:"runInterval,omitempty"`
	Timeout          *metav1.Duration  `json:"timeout,omitempty"`
	ExtraAnnotations map[string]string `json:"extraAnnotations,omitempty"`
	ExtraLabels      map[string]string `json:"extraLabels,omitempty"`
	PodSpec          corev1.PodSpec    `json:"podSpec,omitempty"`
	PodAnnotations   map[string]string `json:"podAnnotations,omitempty"`
	PodLabels        map[string]string `json:"podLabels,omitempty"`
}

// upgradeLegacyPodSpec copies pod configuration from the legacy layout when required.
func upgradeLegacyPodSpec(out *khapi.CheckPodSpec, legacy legacyCheckSpec) {
	if len(out.Spec.Containers) > 0 || len(out.Spec.Volumes) > 0 {
		return
	}

	if len(legacy.PodSpec.Containers) == 0 && len(legacy.PodSpec.Volumes) == 0 {
		return
	}

	out.Spec = legacy.PodSpec

	if len(legacy.PodAnnotations) == 0 && len(legacy.PodLabels) == 0 {
		return
	}

	metadata := &khapi.CheckPodMetadata{}
	if len(legacy.PodAnnotations) > 0 {
		metadata.Annotations = legacy.PodAnnotations
	}
	if len(legacy.PodLabels) > 0 {
		metadata.Labels = legacy.PodLabels
	}
	out.Metadata = metadata
}

// toError creates an AdmissionResponse describing the supplied error in a standard format.
func toError(err error) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result:  &metav1.Status{Message: err.Error()},
	}
}
