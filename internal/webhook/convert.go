package webhook

import (
	"encoding/json"
	"fmt"
	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	log "github.com/sirupsen/logrus"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	"io"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

// Convert handles AdmissionReview requests for legacy Kuberhealthy checks and
// returns a response that upgrades them to the v2 API.
func Convert(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read body: %v", err), http.StatusBadRequest)
		return
	}

	review := admissionv1.AdmissionReview{}
	if err := json.Unmarshal(body, &review); err != nil {
		http.Error(w, fmt.Sprintf("unmarshal review: %v", err), http.StatusBadRequest)
		return
	}

	review.Response = convertReview(&review)
	if review.Request != nil {
		review.Response.UID = review.Request.UID
	}

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

	raw := ar.Request.Object.Raw
	meta := metav1.TypeMeta{}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return toError(fmt.Errorf("parse typemeta: %w", err))
	}

	if meta.APIVersion != "comcast.github.io/v1" {
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	check, warning, err := convertLegacy(raw, meta.Kind)
	if err != nil {
		return toError(err)
	}
	if check == nil {
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

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

func convertLegacy(raw []byte, kind string) (*khapi.KuberhealthyCheck, string, error) {
	switch kind {
	case "KuberhealthyCheck":
		out := khapi.KuberhealthyCheck{}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, "", fmt.Errorf("parse object: %w", err)
		}
		out.APIVersion = "kuberhealthy.github.io/v2"
		return &out, "converted legacy comcast.github.io/v1 KuberhealthyCheck to kuberhealthy.github.io/v2", nil
	default:
		return nil, "", nil
	}
}

func toError(err error) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result:  &metav1.Status{Message: err.Error()},
	}
}
