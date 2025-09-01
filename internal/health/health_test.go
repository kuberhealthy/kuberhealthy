package health

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// TestAddError verifies that AddError appends entries to the state's error slice.
func TestAddError(t *testing.T) {
	t.Parallel()
	s := NewState()
	s.AddError("", "error1", "", "error2")
	if len(s.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(s.Errors))
	}
	if s.Errors[0] != "error1" || s.Errors[1] != "error2" {
		t.Fatalf("unexpected errors slice: %#v", s.Errors)
	}
}

// TestWriteHTTPStatusResponse ensures the state is serialized to JSON with the expected headers and body.
func TestWriteHTTPStatusResponse(t *testing.T) {
	t.Parallel()
	s := State{
		OK:     true,
		Errors: []string{"e1"},
		CheckDetails: map[string]CheckDetail{
			"check": {},
		},
		Metadata: map[string]string{"a": "b"},
	}

	rr := httptest.NewRecorder()
	if err := s.WriteHTTPStatusResponse(rr); err != nil {
		t.Fatalf("WriteHTTPStatusResponse returned error: %v", err)
	}

	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", ct)
	}

	expected, _ := json.MarshalIndent(s, "", "  ")
	if rr.Body.String() != string(expected) {
		t.Fatalf("unexpected body: got %s want %s", rr.Body.String(), string(expected))
	}
}

// TestNewState confirms that NewState returns a healthy state with empty slices and maps initialized.
func TestNewState(t *testing.T) {
	t.Parallel()
	s := NewState()
	if !s.OK {
		t.Errorf("expected OK true, got false")
	}
	if s.Errors == nil || len(s.Errors) != 0 {
		t.Errorf("expected empty error slice, got %#v", s.Errors)
	}
	if s.CheckDetails == nil || len(s.CheckDetails) != 0 {
		t.Errorf("expected empty CheckDetails map, got %#v", s.CheckDetails)
	}
	if s.Metadata == nil || len(s.Metadata) != 0 {
		t.Errorf("expected empty Metadata map, got %#v", s.Metadata)
	}
}
