package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

// SchemeGroupVersion variable for newly added kh state to be added to Kuberhealthy
var SchemeGroupVersion schema.GroupVersion

// ConfigureScheme configures the runtime scheme for use with CRD creation
func ConfigureScheme(GroupName string, GroupVersion string) error {
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}
	var (
		SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
		AddToScheme   = SchemeBuilder.AddToScheme
	)
	return AddToScheme(scheme.Scheme)
}

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&KuberhealthyState{},
		&KuberhealthyStateList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
