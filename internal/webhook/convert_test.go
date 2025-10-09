package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	yaml "sigs.k8s.io/yaml"
)

// loadLegacyDeploymentCheck reads the legacy deployment check manifest used for webhook tests and returns the first YAML document as JSON.
func loadLegacyDeploymentCheck(t *testing.T) []byte {
	// helping the caller re-use the decoded manifest without cluttering the main test body
	t.Helper()

	// locate the shared test manifest so the conversion test uses the real legacy deployment example
	manifestPath := filepath.Join("..", "..", "tests", "khcheck-test-v1-deployment.yaml")
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	// decode only the first YAML document because the later role and service account objects are irrelevant to conversion
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	firstDoc := map[string]any{}
	err = decoder.Decode(&firstDoc)
	require.NoError(t, err)

	// convert the YAML document into JSON so it can be embedded in the AdmissionReview payload
	jsonBytes, err := json.Marshal(firstDoc)
	require.NoError(t, err)
	return jsonBytes
}

// TestConvert upgrades a v1 check to the current API version via the conversion webhook.
func TestConvert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping webhook conversion test in short mode")
	}
	var created []*khapi.HealthCheck
	restore := SetLegacyHandlers(
		func(ctx context.Context, check *khapi.HealthCheck) error {
			created = append(created, check.DeepCopy())
			return nil
		},
		nil,
		nil,
	)
	defer restore()

	ri := metav1.Duration{Duration: 5 * time.Minute}
	to := metav1.Duration{Duration: 2 * time.Minute}
	old := &khapi.HealthCheck{
		TypeMeta:   metav1.TypeMeta{APIVersion: "comcast.github.io/v1", Kind: "KuberhealthyCheck"},
		ObjectMeta: metav1.ObjectMeta{Name: "ssh-check", Namespace: "kuberhealthy"},
		Spec: khapi.HealthCheckSpec{
			RunInterval: &ri,
			Timeout:     &to,
			PodSpec:     khapi.CheckPodSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "ssh-check", Image: "test"}}}},
		},
	}

	oldRaw, err := json.Marshal(old)
	require.NoError(t, err)

	ar := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:    "123",
			Object: runtimeRawExtension(oldRaw),
		},
	}
	body, err := json.Marshal(ar)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	Convert(w, req)

	res := w.Result()
	defer res.Body.Close()
	require.Equal(t, 200, res.StatusCode)

	out := admissionv1.AdmissionReview{}
	err = json.NewDecoder(res.Body).Decode(&out)
	require.NoError(t, err)

	require.True(t, out.Response.Allowed)
	require.Len(t, out.Response.Warnings, 1)
	require.Nil(t, out.Response.Patch)
	require.Len(t, created, 1)
	require.Equal(t, "kuberhealthy.github.io/v2", created[0].APIVersion)
	require.Equal(t, "HealthCheck", created[0].Kind)
	require.Equal(t, old.Spec, created[0].Spec)
}

// TestConvertLegacySpec converts a legacy YAML check definition to the current API version.
func TestConvertLegacySpec(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping legacy webhook conversion test in short mode")
	}
	var created []*khapi.HealthCheck
	restore := SetLegacyHandlers(
		func(ctx context.Context, check *khapi.HealthCheck) error {
			created = append(created, check.DeepCopy())
			return nil
		},
		nil,
		nil,
	)
	defer restore()

	// legacy YAML representing a comcast.github.io/v1 check
	legacy := `apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: ssh-check
  namespace: kuberhealthy
spec:
  runInterval: 5m
  timeout: 2m
  extraAnnotations:
    comcast.com/testAnnotation: test.annotation
  extraLabels:
    testLabel: testLabel
  podSpec:
    containers:
    - name: ssh-check
      image: rjacks161/ssh-check:v1.0.0
      imagePullPolicy: IfNotPresent
      env:
      - name: SSH_PRIVATE_KEY
        value: "CHANGE_ME"
      - name: SSH_USERNAME
        value: "CHANGEME"
      - name: SSH_EXCLUDE_LIST
        value: "CHANGEME1 CHANGEME2"
      resources:
        requests:
          cpu: 10m
          memory: 50Mi
`

	// convert YAML to JSON for the AdmissionReview
	legacyJSON, err := yaml.YAMLToJSON([]byte(legacy))
	require.NoError(t, err)

	ar := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:    "456",
			Object: runtimeRawExtension(legacyJSON),
		},
	}
	body, err := json.Marshal(ar)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	Convert(w, req)

	res := w.Result()
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	out := admissionv1.AdmissionReview{}
	err = json.NewDecoder(res.Body).Decode(&out)
	require.NoError(t, err)

	require.True(t, out.Response.Allowed)
	require.Nil(t, out.Response.Patch)
	require.Len(t, out.Response.Warnings, 1)
	require.Len(t, created, 1)
	require.Equal(t, "kuberhealthy.github.io/v2", created[0].APIVersion)
	require.Equal(t, "HealthCheck", created[0].Kind)
	_, err = json.Marshal(created[0])
	require.NoError(t, err)
}

// TestConvertDeploymentCheckSpec verifies that the legacy deployment check converts to v2 without dropping required pod fields.
func TestConvertDeploymentCheckSpec(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping deployment webhook conversion test in short mode")
	}

	var created []*khapi.HealthCheck
	restore := SetLegacyHandlers(
		func(ctx context.Context, check *khapi.HealthCheck) error {
			created = append(created, check.DeepCopy())
			return nil
		},
		nil,
		nil,
	)
	defer restore()

	legacy := `apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: deployment
  namespace: kuberhealthy
spec:
  runInterval: 10m
  timeout: 15m
  podSpec:
    containers:
      - name: deployment
        image: kuberhealthy/deployment-check:v1.9.1
        imagePullPolicy: IfNotPresent
        env:
          - name: CHECK_DEPLOYMENT_REPLICAS
            value: "4"
          - name: CHECK_DEPLOYMENT_ROLLING_UPDATE
            value: "true"
        resources:
          requests:
            cpu: 25m
            memory: 15Mi
          limits:
            cpu: 1
    restartPolicy: Never
    serviceAccountName: deployment-sa
    terminationGracePeriodSeconds: 60`

	legacyJSON, err := yaml.YAMLToJSON([]byte(legacy))
	require.NoError(t, err)

	review := admissionv1.AdmissionReview{Request: &admissionv1.AdmissionRequest{UID: "789", Object: runtimeRawExtension(legacyJSON)}}
	body, err := json.Marshal(review)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()
	Convert(w, req)

	res := w.Result()
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	out := admissionv1.AdmissionReview{}
	err = json.NewDecoder(res.Body).Decode(&out)
	require.NoError(t, err)

	require.True(t, out.Response.Allowed)
	require.Nil(t, out.Response.Patch)
	require.Len(t, created, 1)

	converted := created[0]
	require.Equal(t, "kuberhealthy.github.io/v2", converted.APIVersion)
	require.Equal(t, "HealthCheck", converted.Kind)
	require.Equal(t, "deployment", converted.Name)
	require.Equal(t, "kuberhealthy", converted.Namespace)

	require.NotNil(t, converted.Spec.RunInterval)
	require.Equal(t, time.Minute*10, converted.Spec.RunInterval.Duration)
	require.NotNil(t, converted.Spec.Timeout)
	require.Equal(t, time.Minute*15, converted.Spec.Timeout.Duration)

	spec := converted.Spec.PodSpec.Spec
	require.Equal(t, corev1.RestartPolicyNever, spec.RestartPolicy)
	require.Equal(t, "deployment-sa", spec.ServiceAccountName)

	require.NotNil(t, spec.TerminationGracePeriodSeconds)
	require.Equal(t, int64(60), *spec.TerminationGracePeriodSeconds)

	require.Len(t, spec.Containers, 1)
	container := spec.Containers[0]
	require.Equal(t, "deployment", container.Name)
	require.Equal(t, "kuberhealthy/deployment-check:v1.9.1", container.Image)
	require.Equal(t, corev1.PullIfNotPresent, container.ImagePullPolicy)

	require.Len(t, container.Env, 2)
	require.Equal(t, "CHECK_DEPLOYMENT_REPLICAS", container.Env[0].Name)
	require.Equal(t, "4", container.Env[0].Value)
	require.Equal(t, "CHECK_DEPLOYMENT_ROLLING_UPDATE", container.Env[1].Name)
	require.Equal(t, "true", container.Env[1].Value)

	cpuRequest := container.Resources.Requests.Cpu()
	require.NotNil(t, cpuRequest)
	require.True(t, cpuRequest.Equal(resource.MustParse("25m")))

	memoryRequest := container.Resources.Requests.Memory()
	require.NotNil(t, memoryRequest)
	require.True(t, memoryRequest.Equal(resource.MustParse("15Mi")))

	cpuLimit := container.Resources.Limits.Cpu()
	require.NotNil(t, cpuLimit)
	require.True(t, cpuLimit.Equal(resource.MustParse("1")))
}

// TestConvertLegacyDeploymentManifest simulates the admission of the published legacy deployment manifest and verifies the webhook upgrades the API group.
func TestConvertLegacyDeploymentManifest(t *testing.T) {
	// skip the expensive fixture work when the test suite runs in short mode
	if testing.Short() {
		t.Skip("skipping legacy manifest conversion test in short mode")
	}
	var created []*khapi.HealthCheck
	var deleted []client.ObjectKey
	restore := SetLegacyHandlers(
		func(ctx context.Context, check *khapi.HealthCheck) error {
			created = append(created, check.DeepCopy())
			return nil
		},
		func(ctx context.Context, namespace, name string) error {
			deleted = append(deleted, client.ObjectKey{Namespace: namespace, Name: name})
			return nil
		},
		func(namespace, name string, deleter legacyDeleterFunc) {
			if deleter != nil {
				_ = deleter(context.Background(), namespace, name)
			}
		},
	)
	defer restore()

	// load the canonical legacy manifest so the behavior matches the user facing sample
	legacyJSON := loadLegacyDeploymentCheck(t)

	// assert that the fixture really represents the legacy group and kind before conversion
	legacyMeta := metav1.TypeMeta{}
	err := json.Unmarshal(legacyJSON, &legacyMeta)
	require.NoError(t, err)
	require.Equal(t, "comcast.github.io/v1", legacyMeta.APIVersion)
	require.Equal(t, "KuberhealthyCheck", legacyMeta.Kind)

	// rewrite the manifest to represent the original legacy kind so conversion must update both apiVersion and kind
	legacyDoc := map[string]any{}
	err = json.Unmarshal(legacyJSON, &legacyDoc)
	require.NoError(t, err)
	legacyDoc["kind"] = "KuberhealthyCheck"
	legacyJSON, err = json.Marshal(legacyDoc)
	require.NoError(t, err)

	// build a minimal AdmissionReview payload targeting the legacy document
	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:    "legacy-deployment-manifest",
			Object: runtimeRawExtension(legacyJSON),
		},
	}
	body, err := json.Marshal(review)
	require.NoError(t, err)

	// execute the webhook handler exactly like the Kubernetes API server would
	req := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	resp := httptest.NewRecorder()
	Convert(resp, req)

	// decode the response and confirm the webhook accepted and mutated the resource
	result := resp.Result()
	defer result.Body.Close()
	require.Equal(t, http.StatusOK, result.StatusCode)
	convertedReview := admissionv1.AdmissionReview{}
	err = json.NewDecoder(result.Body).Decode(&convertedReview)
	require.NoError(t, err)
	require.NotNil(t, convertedReview.Response)
	require.True(t, convertedReview.Response.Allowed)
	require.Nil(t, convertedReview.Response.Patch)
	require.Len(t, convertedReview.Response.Warnings, 1)

	require.Len(t, created, 1)
	require.Equal(t, "kuberhealthy.github.io/v2", created[0].APIVersion)
	require.Equal(t, "HealthCheck", created[0].Kind)
	require.Equal(t, "deployment", created[0].Name)
	require.Len(t, deleted, 1)
	require.Equal(t, client.ObjectKey{Namespace: "kuberhealthy", Name: "deployment"}, deleted[0])
}

// TestLegacyConversionCreatesModernResource verifies that applying a legacy manifest produces a modern resource and triggers legacy cleanup.
func TestLegacyConversionCreatesModernResource(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping legacy conversion creation test in short mode")
	}

	legacyJSON := loadLegacyDeploymentCheck(t)

	var created []*khapi.HealthCheck
	var deleted []client.ObjectKey
	restore := SetLegacyHandlers(
		func(ctx context.Context, check *khapi.HealthCheck) error {
			created = append(created, check.DeepCopy())
			return nil
		},
		func(ctx context.Context, namespace, name string) error {
			deleted = append(deleted, client.ObjectKey{Namespace: namespace, Name: name})
			return nil
		},
		func(namespace, name string, deleter legacyDeleterFunc) {
			if deleter == nil {
				return
			}
			_ = deleter(context.Background(), namespace, name)
		},
	)
	defer restore()

	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:      "legacy-convert-create",
			Resource: metav1.GroupVersionResource{Group: "comcast.github.io", Version: "v1", Resource: "kuberhealthychecks"},
			Object:   runtimeRawExtension(legacyJSON),
		},
	}
	body, err := json.Marshal(review)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	resp := httptest.NewRecorder()
	Convert(resp, req)

	result := resp.Result()
	defer result.Body.Close()
	require.Equal(t, http.StatusOK, result.StatusCode)

	out := admissionv1.AdmissionReview{}
	err = json.NewDecoder(result.Body).Decode(&out)
	require.NoError(t, err)
	require.NotNil(t, out.Response)
	require.True(t, out.Response.Allowed)
	require.Nil(t, out.Response.Patch)

	require.Len(t, created, 1)
	require.Equal(t, "kuberhealthy.github.io/v2", created[0].APIVersion)
	require.Equal(t, "HealthCheck", created[0].Kind)
	require.Equal(t, "deployment", created[0].Name)
	require.Equal(t, "kuberhealthy", created[0].Namespace)

	require.Len(t, deleted, 1)
	require.Equal(t, client.ObjectKey{Namespace: "kuberhealthy", Name: "deployment"}, deleted[0])
}

// TestConvertLegacyWithoutTypeMeta ensures conversion succeeds when the incoming payload lacks TypeMeta fields but the admission request describes the legacy resource.
func TestConvertLegacyWithoutTypeMeta(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping legacy conversion without typemeta test in short mode")
	}

	var created []*khapi.HealthCheck
	restore := SetLegacyHandlers(
		func(ctx context.Context, check *khapi.HealthCheck) error {
			created = append(created, check.DeepCopy())
			return nil
		},
		nil,
		nil,
	)
	defer restore()

	legacyJSON := loadLegacyDeploymentCheck(t)
	var doc map[string]any
	err := json.Unmarshal(legacyJSON, &doc)
	require.NoError(t, err)
	delete(doc, "apiVersion")
	delete(doc, "kind")
	noMetaJSON, err := json.Marshal(doc)
	require.NoError(t, err)

	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:      "legacy-no-meta",
			Resource: metav1.GroupVersionResource{Group: "comcast.github.io", Version: "v1", Resource: "khchecks"},
			Object:   runtimeRawExtension(noMetaJSON),
		},
	}
	body, err := json.Marshal(review)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	resp := httptest.NewRecorder()
	Convert(resp, req)

	result := resp.Result()
	defer result.Body.Close()
	require.Equal(t, http.StatusOK, result.StatusCode)

	convertedReview := admissionv1.AdmissionReview{}
	err = json.NewDecoder(result.Body).Decode(&convertedReview)
	require.NoError(t, err)
	require.NotNil(t, convertedReview.Response)
	require.True(t, convertedReview.Response.Allowed)
	require.Nil(t, convertedReview.Response.Patch)
	require.Len(t, created, 1)
	require.Equal(t, "kuberhealthy.github.io/v2", created[0].APIVersion)
	require.Equal(t, "HealthCheck", created[0].Kind)
}

// TestLegacyDeleteBypass ensures delete admissions skip conversion and do not invoke creation handlers.
func TestLegacyDeleteBypass(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping legacy delete bypass test in short mode")
	}

	legacyJSON := loadLegacyDeploymentCheck(t)

	var created bool
	restore := SetLegacyHandlers(
		func(ctx context.Context, check *khapi.HealthCheck) error {
			created = true
			return nil
		},
		nil,
		nil,
	)
	defer restore()

	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:       "legacy-delete-bypass",
			Operation: admissionv1.Delete,
			Resource:  metav1.GroupVersionResource{Group: "comcast.github.io", Version: "v1", Resource: "khchecks"},
			OldObject: runtimeRawExtension(legacyJSON),
		},
	}
	body, err := json.Marshal(review)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	resp := httptest.NewRecorder()
	Convert(resp, req)

	result := resp.Result()
	defer result.Body.Close()
	require.Equal(t, http.StatusOK, result.StatusCode)

	converted := admissionv1.AdmissionReview{}
	err = json.NewDecoder(result.Body).Decode(&converted)
	require.NoError(t, err)
	require.NotNil(t, converted.Response)
	require.True(t, converted.Response.Allowed)
	require.Nil(t, converted.Response.Patch)

	require.False(t, created, "delete admission should not trigger creation")
}

// TestConvertLegacyResourceNames verifies the webhook upgrades every legacy resource alias to the modern API group.
func TestConvertLegacyResourceNames(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping legacy resource alias conversion test in short mode")
	}

	legacyJSON := loadLegacyDeploymentCheck(t)
	aliases := []string{"khc", "khcheck", "kuberhealthycheck", "khchecks", "kuberhealthychecks"}

	for _, alias := range aliases {
		// capture range variable for the subtest closure
		alias := alias
		t.Run(alias, func(t *testing.T) {
			var created []*khapi.HealthCheck
			restore := SetLegacyHandlers(
				func(ctx context.Context, check *khapi.HealthCheck) error {
					created = append(created, check.DeepCopy())
					return nil
				},
				nil,
				nil,
			)
			defer restore()

			review := admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					UID: types.UID("legacy-alias-" + alias),
					Resource: metav1.GroupVersionResource{
						Group:    "comcast.github.io",
						Version:  "v1",
						Resource: alias,
					},
					Object: runtimeRawExtension(legacyJSON),
				},
			}

			body, err := json.Marshal(review)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
			resp := httptest.NewRecorder()
			Convert(resp, req)

			result := resp.Result()
			t.Cleanup(func() { result.Body.Close() })
			require.Equal(t, http.StatusOK, result.StatusCode)

			convertedReview := admissionv1.AdmissionReview{}
			err = json.NewDecoder(result.Body).Decode(&convertedReview)
			require.NoError(t, err)
			require.NotNil(t, convertedReview.Response)
			require.True(t, convertedReview.Response.Allowed)
			require.Nil(t, convertedReview.Response.Patch)
			require.Len(t, created, 1)
			require.Equal(t, "kuberhealthy.github.io/v2", created[0].APIVersion)
			require.Equal(t, "HealthCheck", created[0].Kind)
		})
	}
}

// TestLegacyWebhookResourceCoverage ensures the mutating webhook watches every legacy resource alias so legacy manifests are always converted.
func TestLegacyWebhookResourceCoverage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping webhook manifest coverage test in short mode")
	}

	manifestPath := filepath.Join("..", "..", "deploy", "base", "mutatingwebhook.yaml")
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var cfg struct {
		Webhooks []struct {
			Rules []struct {
				Operations []string `yaml:"operations"`
				Resources  []string `yaml:"resources"`
			} `yaml:"rules"`
		} `yaml:"webhooks"`
	}
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)
	require.NotEmpty(t, cfg.Webhooks)
	require.NotEmpty(t, cfg.Webhooks[0].Rules)

	needed := map[string]bool{
		"khc":                false,
		"khcheck":            false,
		"kuberhealthycheck":  false,
		"khchecks":           false,
		"kuberhealthychecks": false,
	}
	var wildcard bool

	for _, rule := range cfg.Webhooks[0].Rules {
		ops := map[string]bool{}
		for _, op := range rule.Operations {
			ops[strings.ToUpper(op)] = true
		}
		require.True(t, ops["CREATE"], "legacy webhook missing CREATE operation")
		require.True(t, ops["UPDATE"], "legacy webhook missing UPDATE operation")

		for _, res := range rule.Resources {
			if res == "*" {
				wildcard = true
				continue
			}
			if _, ok := needed[res]; ok {
				needed[res] = true
			}
		}
	}

	if !wildcard {
		for name, found := range needed {
			require.Truef(t, found, "legacy resource %s missing from mutating webhook", name)
		}
	}
}

// runtimeRawExtension wraps a byte slice inside a RawExtension for AdmissionReview payloads.
func runtimeRawExtension(b []byte) runtime.RawExtension {
	return runtime.RawExtension{Raw: b}
}
