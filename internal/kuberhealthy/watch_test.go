package kuberhealthy

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestConvertToKHCheckNamespace ensures that a converted healthcheck preserves its namespace.
func TestConvertToKHCheckNamespace(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kuberhealthy.github.io/v2",
		"kind":       "HealthCheck",
		"metadata": map[string]interface{}{
			"name": "example",
		},
	}}
	u.SetNamespace("custom-ns")

	khc, err := convertToKHCheck(u)
	if err != nil {
		t.Fatalf("convertToKHCheck returned error: %v", err)
	}
	if khc.Namespace != "custom-ns" {
		t.Errorf("expected namespace custom-ns, got %s", khc.Namespace)
	}
}
