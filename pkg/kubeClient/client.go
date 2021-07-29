package kubeClient // import "github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"

import (
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Create returns a kubernetes api clientset that enables communication with
// the kubernetes API via the internal service.
func Create(kubeConfigFile string) (*kubernetes.Clientset, error) {
	kubeconfig, err := rest.InClusterConfig()
	if err != nil {
		// If not in cluster, use kube config file
		kubeconfig, err = clientcmd.BuildConfigFromFlags("", kubeConfigFile)
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfig(kubeconfig)
}
