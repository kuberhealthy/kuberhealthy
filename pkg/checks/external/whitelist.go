package external

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Comcast/kuberhealthy/pkg/khcheckcrd"
)

// GetWhitelistedUUIDForExternalCheck fetches the current allowed UUID for an
// external check.  This data is stored in khcheck custom resources.
func GetWhitelistedUUIDForExternalCheck(checkName string) (string, error) {
	// make a new crd check client
	checkClient, err := khcheckcrd.Client(checkCRDGroup, checkCRDVersion, kubeConfigFile)
	if err != nil {
		return "", err
	}

	r, err := checkClient.Get(metav1.GetOptions{}, checkCRDResource, checkName)
	if err != nil {
		return "", err
	}

	return r.Spec.CurrentUUID, nil
}
