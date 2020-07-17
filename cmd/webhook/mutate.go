package main

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/admission/v1beta1"
	// "github.com/Comcast/kuberhealthy/v2/pkg/khcheckcrd"
)

func (wh *webhook) mutate(review *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	log.Infoln("Received a mutation request for", review.Request.Name, khchecks, "from", review.Request.Namespace, "namespace.", "Request:", review.Request.UID+".")
	if review.Kind != khchecks {
		log.Errorln("Skipping mutation request for", review.Request.UID, "because it is not a khcheck.")
		return nil
	}

	var KuberhealthyCheck khc
	err := json.Unmarshal(review.Request.Object.Raw, &khc)
	if err != nil {
		log.Errorln("Failed to unmarshal khcheck:", err.Error())
		return nil
	}

	return nil
}
