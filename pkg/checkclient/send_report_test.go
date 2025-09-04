package checkclient

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kuberhealthy/kuberhealthy/v3/internal/envs"
	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
)

// TestReportSuccessAndFailure verifies success and failure reports send expected JSON and headers.
func TestReportSuccessAndFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	tests := []struct {
		name     string
		call     func() error
		expected khapi.KuberhealthyCheckStatus
	}{
		{
			name: "success",
			call: ReportSuccess,
			expected: khapi.KuberhealthyCheckStatus{
				OK:     true,
				Errors: []string{},
			},
		},
		{
			name: "failure",
			call: func() error { return ReportFailure([]string{"err1", "err2"}) },
			expected: khapi.KuberhealthyCheckStatus{
				OK:     false,
				Errors: []string{"err1", "err2"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotHeader string
			var gotBody []byte

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotHeader = r.Header.Get("kh-run-uuid")
				var err error
				gotBody, err = io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed reading body: %v", err)
				}
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			uuid := "test-uuid"
			t.Setenv(envs.KHReportingURL, srv.URL)
			t.Setenv(envs.KHRunUUID, uuid)

			if err := tc.call(); err != nil {
				t.Fatalf("call returned error: %v", err)
			}

			if gotHeader != uuid {
				t.Fatalf("kh-run-uuid header = %q, want %q", gotHeader, uuid)
			}

			var status khapi.KuberhealthyCheckStatus
			if err := json.Unmarshal(gotBody, &status); err != nil {
				t.Fatalf("failed to unmarshal body: %v", err)
			}
			if status.OK != tc.expected.OK {
				t.Fatalf("status OK = %v, want %v", status.OK, tc.expected.OK)
			}
			if len(status.Errors) != len(tc.expected.Errors) {
				t.Fatalf("errors length = %d, want %d", len(status.Errors), len(tc.expected.Errors))
			}
			for i := range status.Errors {
				if status.Errors[i] != tc.expected.Errors[i] {
					t.Fatalf("error[%d] = %q, want %q", i, status.Errors[i], tc.expected.Errors[i])
				}
			}
		})
	}
}

// TestSendReportRetry ensures ReportSuccess retries once after an initial server error.
func TestSendReportRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	var reqs int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs++
		if reqs == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	uuid := "retry-uuid"
	t.Setenv(envs.KHReportingURL, srv.URL)
	t.Setenv(envs.KHRunUUID, uuid)

	if err := ReportSuccess(); err != nil {
		t.Fatalf("ReportSuccess returned error: %v", err)
	}

	if reqs != 2 {
		t.Fatalf("expected 2 requests, got %d", reqs)
	}
}
