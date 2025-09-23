package kuberhealthy

import (
	"fmt"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// convertToKHCheck normalizes informer objects into concrete KuberhealthyCheck resources.
func convertToKHCheck(obj interface{}) (*khapi.KuberhealthyCheck, error) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		// handles delete events delivered as tombstones when objects disappear rapidly
		obj = tombstone.Obj
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("unexpected object type %T", obj)
	}
	khc := &khapi.KuberhealthyCheck{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, khc)
	if err != nil {
		return nil, err
	}
	// ensure the converted object retains its original namespace
	khc.Namespace = u.GetNamespace()
	khc.EnsureCreationTimestamp()
	return khc, nil
}
