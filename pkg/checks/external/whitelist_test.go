package external

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	khstatev1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khstate/v1"
)

// GetWhitelistedUUIDForExternalCheck fetches the current allowed UUID for an
// external check.  This data is stored in khcheck custom resources.
func GetWhitelistedUUIDForExternalCheck(checkNamespace string, checkName string) (string, error) {
	// make a new crd check client
	stateClient, err := khstatev1.Client(kubeConfigFile)
	if err != nil {
		return "", err
	}

	r, err := stateClient.KuberhealthyStates(checkNamespace).Get(checkName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return r.Spec.CurrentUUID, nil
}
