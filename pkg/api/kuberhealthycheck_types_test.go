package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
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
