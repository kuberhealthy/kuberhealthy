//go:build integration
// +build integration

package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestGetCheckSetsCreationTimestamp ensures GetCheck populates metadata.CreationTimestamp when missing.
func TestGetCheckSetsCreationTimestamp(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, AddToScheme(scheme))

	check := &HealthCheck{
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
