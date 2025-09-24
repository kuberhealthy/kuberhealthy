package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	log "github.com/sirupsen/logrus"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	review.Response = convertReview(&review)
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
func convertReview(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	if ar.Request == nil {
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	// read the raw request and inspect the type information of the resource
	raw := ar.Request.Object.Raw
	meta := metav1.TypeMeta{}
	err := json.Unmarshal(raw, &meta)
	if err != nil {
		return toError(fmt.Errorf("parse typemeta: %w", err))
	}

	if meta.APIVersion != "comcast.github.io/v1" {
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	// attempt to convert the incoming legacy object into the modern representation
	check, warning, err := convertLegacy(raw, meta.Kind)
	if err != nil {
		return toError(err)
	}
	if check == nil {
		return &admissionv1.AdmissionResponse{Allowed: true}
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
		return toError(fmt.Errorf("marshal patch: %w", err))
	}

	pt := admissionv1.PatchTypeJSONPatch
	return &admissionv1.AdmissionResponse{
		Allowed:   true,
		Patch:     patchBytes,
		PatchType: &pt,
		Warnings:  []string{warning},
	}
}

// convertLegacy upgrades a legacy Kuberhealthy object into a modern v2 check when supported.
func convertLegacy(raw []byte, kind string) (*khapi.KuberhealthyCheck, string, error) {
	switch kind {
	case "KuberhealthyCheck":
		// decode the legacy object and rewrite the API version to the current value
		out := khapi.KuberhealthyCheck{}
		err := json.Unmarshal(raw, &out)
		if err != nil {
			return nil, "", fmt.Errorf("parse object: %w", err)
		}
		out.APIVersion = "kuberhealthy.github.io/v2"
		return &out, "converted legacy comcast.github.io/v1 KuberhealthyCheck to kuberhealthy.github.io/v2", nil
	default:
		return nil, "", nil
	}
}

// toError creates an AdmissionResponse describing the supplied error in a standard format.
func toError(err error) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result:  &metav1.Status{Message: err.Error()},
	}
}
