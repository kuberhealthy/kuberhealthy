package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestSpecDoesNotExposeCreationTimestamp ensures creationTimestamp is not serialized in pod metadata.
func TestSpecDoesNotExposeCreationTimestamp(t *testing.T) {
	t.Parallel()

	check := &KuberhealthyCheck{
		Spec: KuberhealthyCheckSpec{
			PodSpec: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"foo": "bar"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "test",
						Image: "busybox",
					}},
				},
			},
		},
	}

	raw, err := json.Marshal(check)
	require.NoError(t, err)

	var data map[string]any
	require.NoError(t, json.Unmarshal(raw, &data))

	spec := data["spec"].(map[string]any)
	podSpec := spec["podSpec"].(map[string]any)
	metadata := podSpec["metadata"].(map[string]any)
	if v, found := metadata["creationTimestamp"]; found {
		require.Nil(t, v, "creationTimestamp must be nil in spec.podSpec.metadata")
	}
}
