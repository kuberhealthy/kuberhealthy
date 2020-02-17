package external

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Comcast/kuberhealthy/v2/pkg/khstatecrd"
)

// GetWhitelistedUUIDForExternalCheck fetches the current allowed UUID for an
// external check.  This data is stored in khcheck custom resources.
func GetWhitelistedUUIDForExternalCheck(checkNamespace string, checkName string) (string, error) {
	// make a new crd check client
	stateClient, err := khstatecrd.Client(CRDGroup, CRDVersion, kubeConfigFile, checkNamespace)
	if err != nil {
		return "", err
	}

	r, err := stateClient.Get(metav1.GetOptions{}, stateCRDResource, checkNamespace, checkName)
	if err != nil {
		return "", err
	}

	return r.Spec.CurrentUUID, nil
}
