package jobwebhook

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Convert handles ConversionReview requests for legacy KuberhealthyJob
// objects and returns a response containing converted v2
// KuberhealthyCheck resources.
func Convert(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read body: %v", err), http.StatusBadRequest)
		return
	}

	review := apiextv1.ConversionReview{}
	if err := json.Unmarshal(body, &review); err != nil {
		http.Error(w, fmt.Sprintf("unmarshal review: %v", err), http.StatusBadRequest)
		return
	}
	if review.Request == nil {
		http.Error(w, "missing request", http.StatusBadRequest)
		return
	}

	resp := &apiextv1.ConversionResponse{UID: review.Request.UID}
	for _, obj := range review.Request.Objects {
		job := khJob{}
		if err := json.Unmarshal(obj.Raw, &job); err != nil {
			resp.Result = metav1.Status{Message: fmt.Sprintf("parse job: %v", err)}
			review.Response = resp
			writeReview(w, &review)
			return
		}

		podSpec := khapi.CheckPodSpec{Spec: job.Spec.PodSpec.Spec}
		if len(job.Spec.PodSpec.ObjectMeta.Labels) > 0 || len(job.Spec.PodSpec.ObjectMeta.Annotations) > 0 {
			podSpec.Metadata = &khapi.CheckPodMetadata{
				Labels:      job.Spec.PodSpec.ObjectMeta.Labels,
				Annotations: job.Spec.PodSpec.ObjectMeta.Annotations,
			}
		}

		check := khapi.KuberhealthyCheck{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "kuberhealthy.github.io/v2",
				Kind:       "KuberhealthyCheck",
			},
			ObjectMeta: job.ObjectMeta,
			Spec: khapi.KuberhealthyCheckSpec{
				SingleRun:        job.Spec.RunOnce,
				RunInterval:      job.Spec.RunInterval,
				Timeout:          job.Spec.Timeout,
				ExtraAnnotations: job.Spec.ExtraAnnotations,
				ExtraLabels:      job.Spec.ExtraLabels,
				PodSpec:          podSpec,
			},
		}

		raw, err := json.Marshal(check)
		if err != nil {
			resp.Result = metav1.Status{Message: fmt.Sprintf("marshal check: %v", err)}
			review.Response = resp
			writeReview(w, &review)
			return
		}
		resp.ConvertedObjects = append(resp.ConvertedObjects, runtime.RawExtension{Raw: raw})
	}
	resp.Result.Status = metav1.StatusSuccess
	review.Response = resp
	writeReview(w, &review)
}

func writeReview(w http.ResponseWriter, review *apiextv1.ConversionReview) {
	w.Header().Set("Content-Type", "application/json")
	respBytes, err := json.Marshal(review)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshal response: %v", err), http.StatusInternalServerError)
		return
	}
	_, err = w.Write(respBytes)
	if err != nil {
		log.Errorln("write response:", err)
	}
}

type khJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              khJobSpec `json:"spec,omitempty"`
}

type khJobSpec struct {
	RunOnce          bool                   `json:"runOnce,omitempty"`
	RunInterval      *metav1.Duration       `json:"runInterval,omitempty"`
	Timeout          *metav1.Duration       `json:"timeout,omitempty"`
	ExtraAnnotations map[string]string      `json:"extraAnnotations,omitempty"`
	ExtraLabels      map[string]string      `json:"extraLabels,omitempty"`
	PodSpec          corev1.PodTemplateSpec `json:"podSpec,omitempty"`
}
