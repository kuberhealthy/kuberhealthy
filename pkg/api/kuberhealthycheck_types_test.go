package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGetCheckSetsCreationTimestamp ensures GetCheck populates metadata.CreationTimestamp when missing.
func TestGetCheckSetsCreationTimestamp(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, AddToScheme(scheme))

	check := &KuberhealthyCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(check).Build()
	nn := types.NamespacedName{Name: "test", Namespace: "default"}

	out, err := GetCheck(context.Background(), cl, nn)
	require.NoError(t, err)
	require.False(t, out.CreationTimestamp.IsZero(), "creation timestamp must be set")
}

// TestSpecDoesNotExposeCreationTimestamp ensures creationTimestamp is not serialized in pod metadata.
func TestSpecDoesNotExposeCreationTimestamp(t *testing.T) {
	t.Parallel()

	check := &KuberhealthyCheck{
		Spec: KuberhealthyCheckSpec{
			PodSpec: CheckPodSpec{
				Metadata: &CheckPodMetadata{Labels: map[string]string{"foo": "bar"}},
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
	_, found := metadata["creationTimestamp"]
	require.False(t, found, "creationTimestamp must be absent in spec.podSpec.metadata")

}

// TestSingleRunOnlyRoundTrip ensures singleRunOnly marshals and unmarshals correctly.
func TestSingleRunOnlyRoundTrip(t *testing.T) {
	t.Parallel()

	check := KuberhealthyCheck{Spec: KuberhealthyCheckSpec{SingleRun: true}}

	raw, err := json.Marshal(check)
	require.NoError(t, err)
	require.Contains(t, string(raw), "\"singleRunOnly\":true")

	var out KuberhealthyCheck
	require.NoError(t, json.Unmarshal(raw, &out))
	require.True(t, out.Spec.SingleRun)
}

// TestSingleRunOnlyOmittedWhenFalse ensures singleRunOnly is absent when false.
func TestSingleRunOnlyOmittedWhenFalse(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(KuberhealthyCheck{})
	require.NoError(t, err)
	require.NotContains(t, string(raw), "singleRunOnly")

	var out KuberhealthyCheck
	require.NoError(t, json.Unmarshal(raw, &out))
	require.False(t, out.Spec.SingleRun)
}
