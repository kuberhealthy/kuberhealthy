package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kuberhealthy/kuberhealthy/v3/internal/health"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/kuberhealthy"
	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestCheckReportHandler validates the check report handler logic.
func TestCheckReportHandler(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping check report handler test in short mode")
	}
	// preserve original function implementations
	origValidateHeader := validateUsingRequestHeaderFunc
	origStore := storeCheckStateFunc
	origClient := Globals.khClient
	origKH := Globals.kh
	t.Parallel()
	defer func() {
		validateUsingRequestHeaderFunc = origValidateHeader
		storeCheckStateFunc = origStore
		Globals.khClient = origClient
		Globals.kh = origKH
	}()
	Globals.khClient = nil
	Globals.kh = nil

	// valid report is persisted and returns HTTP 200
	t.Run("valid report", func(t *testing.T) {
		validateUsingRequestHeaderFunc = func(ctx context.Context, r *http.Request) (PodReportInfo, bool, error) {
			return PodReportInfo{Name: "my-check", Namespace: "my-namespace", UUID: "abc"}, true, nil
		}
		var storedName, storedNamespace string
		var storedDetails *khapi.KuberhealthyCheckStatus
		storeCheckStateFunc = func(_ client.Client, name, namespace string, details *khapi.KuberhealthyCheckStatus) error {
			storedName = name
			storedNamespace = namespace
			storedDetails = details
			return nil
		}

		report := health.Report{OK: true}
		b, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("failed to marshal report: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		if err := checkReportHandler(rr, req); err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if storedName != "my-check" || storedNamespace != "my-namespace" {
			t.Fatalf("storeCheckState called with unexpected values: %s %s", storedName, storedNamespace)
		}
		if storedDetails == nil || !storedDetails.OK {
			t.Fatalf("storeCheckState received incorrect details: %+v", storedDetails)
		}
	})

	// a report lacking error details while not OK results in a bad request
	t.Run("missing error when not OK", func(t *testing.T) {
		validateUsingRequestHeaderFunc = func(ctx context.Context, r *http.Request) (PodReportInfo, bool, error) {
			return PodReportInfo{Name: "my-check", Namespace: "my-namespace", UUID: "abc"}, true, nil
		}
		storeCalled := false
		storeCheckStateFunc = func(client.Client, string, string, *khapi.KuberhealthyCheckStatus) error {
			storeCalled = true
			return nil
		}

		report := health.Report{OK: false}
		b, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("failed to marshal report: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		if err := checkReportHandler(rr, req); err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
		if storeCalled {
			t.Fatalf("storeCheckState should not be called on invalid report")
		}
	})

	// providing errors while OK leads to a bad request and no state storage
	t.Run("errors present when OK", func(t *testing.T) {
		validateUsingRequestHeaderFunc = func(ctx context.Context, r *http.Request) (PodReportInfo, bool, error) {
			return PodReportInfo{Name: "my-check", Namespace: "my-namespace", UUID: "abc"}, true, nil
		}
		storeCalled := false
		storeCheckStateFunc = func(client.Client, string, string, *khapi.KuberhealthyCheckStatus) error {
			storeCalled = true
			return nil
		}

		report := health.Report{OK: true, Errors: []string{"error"}}
		b, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("failed to marshal report: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		if err := checkReportHandler(rr, req); err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
		if storeCalled {
			t.Fatalf("storeCheckState should not be called on invalid report")
		}
	})

	t.Run("reject report after timeout", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, khapi.AddToScheme(scheme))

		started := time.Now().Add(-time.Minute)
		timeout := time.Second

		check := &khapi.KuberhealthyCheck{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "expired-check",
				Namespace:       "default",
				ResourceVersion: "1",
			},
			Spec: khapi.KuberhealthyCheckSpec{
				Timeout: &metav1.Duration{Duration: timeout},
			},
			Status: khapi.KuberhealthyCheckStatus{
				LastRunUnix: started.Unix(),
				CurrentUUID: "expired-uuid",
				OK:          true,
			},
		}

		cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(check).WithStatusSubresource(check).Build()
		Globals.khClient = cl
		Globals.kh = kuberhealthy.New(context.Background(), cl)
		t.Cleanup(func() {
			Globals.khClient = nil
			Globals.kh = nil
		})

		validateUsingRequestHeaderFunc = func(ctx context.Context, r *http.Request) (PodReportInfo, bool, error) {
			return PodReportInfo{Name: check.Name, Namespace: check.Namespace, UUID: check.CurrentUUID()}, true, nil
		}
		storeCalled := false
		storeCheckStateFunc = func(client.Client, string, string, *khapi.KuberhealthyCheckStatus) error {
			storeCalled = true
			return nil
		}

		report := health.Report{OK: true}
		b, err := json.Marshal(report)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		require.NoError(t, checkReportHandler(rr, req))
		require.Equal(t, http.StatusGone, rr.Code)
		require.False(t, storeCalled)
	})
}
