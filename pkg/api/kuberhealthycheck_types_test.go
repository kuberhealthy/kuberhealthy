package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

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
