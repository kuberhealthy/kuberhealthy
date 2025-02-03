package external

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	khClient "github.com/kuberhealthy/kuberhealthy/v4/pkg/generated/clientset/versioned"
)

// GetWhitelistedUUIDForExternalCheck fetches the current allowed UUID for an
// external check.  This data is stored in khcheck custom resources.
func GetWhitelistedUUIDForExternalCheck(ctx context.Context, checkNamespace string, checkName string) (string, error) {
	// make a new crd check client
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return "", err
	}
	KuberhealthyClient, err := khClient.NewForConfig(restConfig)
	if err != nil {
		return "", err
	}

	r, err := KuberhealthyClient.ComcastV1().KuberhealthyStates(checkNamespace).Get(ctx, checkName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return r.Spec.CurrentUUID, nil
}
