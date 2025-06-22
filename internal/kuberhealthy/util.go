package kuberhealthy

import "k8s.io/apimachinery/pkg/types"

// createNamespacedName is a shortcut to making a types.NamespacedName from a name and namespace
func createNamespacedName(name string, namespace string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
}
