package kubeclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ForegroundDeleteOption implements the client.DeleteOption interface.
func ForegroundDeleteOption() client.DeleteOption {
	propagationPolicy := metav1.DeletePropagationForeground
	return client.PropagationPolicy(propagationPolicy)
}
