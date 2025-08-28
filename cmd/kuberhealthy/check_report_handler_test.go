package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kuberhealthy/kuberhealthy/v3/internal/health"
	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
)

// TestCheckReportHandler validates the check report handler logic.
func TestCheckReportHandler(t *testing.T) {
	// preserve original function implementations
	origValidateHeader := validateUsingRequestHeaderFunc
	origValidateIP := validatePodReportBySourceIPFunc
	origStore := storeCheckStateFunc
	t.Parallel()
	defer func() {
		validateUsingRequestHeaderFunc = origValidateHeader
		validatePodReportBySourceIPFunc = origValidateIP
		storeCheckStateFunc = origStore
	}()

	t.Run("valid report", func(t *testing.T) {
		validateUsingRequestHeaderFunc = func(ctx context.Context, r *http.Request) (PodReportInfo, bool, error) {
			return PodReportInfo{Name: "my-check", Namespace: "my-namespace", UUID: "abc"}, true, nil
		}
		validatePodReportBySourceIPFunc = func(ctx context.Context, r *http.Request) (PodReportInfo, error) {
			t.Fatalf("unexpected call to validatePodReportBySourceIPFunc")
			return PodReportInfo{}, nil
		}
		var storedName, storedNamespace string
		var storedDetails *khapi.KuberhealthyCheckStatus
		storeCheckStateFunc = func(name, namespace string, details *khapi.KuberhealthyCheckStatus) error {
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

	t.Run("missing error when not OK", func(t *testing.T) {
		validateUsingRequestHeaderFunc = func(ctx context.Context, r *http.Request) (PodReportInfo, bool, error) {
			return PodReportInfo{Name: "my-check", Namespace: "my-namespace", UUID: "abc"}, true, nil
		}
		validatePodReportBySourceIPFunc = func(ctx context.Context, r *http.Request) (PodReportInfo, error) {
			t.Fatalf("unexpected call to validatePodReportBySourceIPFunc")
			return PodReportInfo{}, nil
		}
		storeCalled := false
		storeCheckStateFunc = func(string, string, *khapi.KuberhealthyCheckStatus) error {
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

	t.Run("errors present when OK", func(t *testing.T) {
		validateUsingRequestHeaderFunc = func(ctx context.Context, r *http.Request) (PodReportInfo, bool, error) {
			return PodReportInfo{Name: "my-check", Namespace: "my-namespace", UUID: "abc"}, true, nil
		}
		validatePodReportBySourceIPFunc = func(ctx context.Context, r *http.Request) (PodReportInfo, error) {
			t.Fatalf("unexpected call to validatePodReportBySourceIPFunc")
			return PodReportInfo{}, nil
		}
		storeCalled := false
		storeCheckStateFunc = func(string, string, *khapi.KuberhealthyCheckStatus) error {
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
}
