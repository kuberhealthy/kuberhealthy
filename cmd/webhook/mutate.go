package main

import (
	"encoding/json"
	"errors"

	"github.com/Comcast/kuberhealthy/v2/pkg/khcheckcrd"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (wh *webhook) mutate(review *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	log.Infoln("Received a mutation request for", review.Request.Name, khchecks, "from", review.Request.Namespace, "namespace.", "Request:", review.Request.UID+".")
	if review.Kind != khchecks {
		err := errors.New("Skipping mutation request for " + string(review.Request.UID) + " because it is not a khcheck.")
		log.Errorln(err.Error())
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	var khc khcheckcrd.KuberhealthyCheck
	err := json.Unmarshal(review.Request.Object.Raw, &khc)
	if err != nil {
		err := errors.New("Failed to unmarshal khcheck: " + err.Error())
		log.Errorln(err.Error())
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	allowed := true
	result := &metav1.Status{}
	if khc.Spec.PodSpec.Containers == nil || len(khc.Spec.PodSpec.Containers) == 0 {
		allowed = false
		result.Reason = "Kuberhealthy check spec.podSpec.containers is not an array or list of containers."
	}

	return &v1beta1.AdmissionResponse{
		Allowed: allowed,
		// Patch:   patchBytes,
		// PatchType: func() *v1beta1.PatchType {
		// 	patch := v1beta1.PatchTypeJSONPatch
		// 	return &patch
		// }(),
		Result: result,
	}
}
