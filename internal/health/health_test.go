package health

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	kuberhealthycheckv2 "github.com/kuberhealthy/crds/api/v2"
)

func TestAddError(t *testing.T) {
	s := NewState()
	s.AddError("", "error1", "", "error2")
	if len(s.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(s.Errors))
	}
	if s.Errors[0] != "error1" || s.Errors[1] != "error2" {
		t.Fatalf("unexpected errors slice: %#v", s.Errors)
	}
}

func TestWriteHTTPStatusResponse(t *testing.T) {
	s := State{
		OK:     true,
		Errors: []string{"e1"},
		CheckDetails: map[string]kuberhealthycheckv2.KuberhealthyCheckStatus{
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

func TestNewState(t *testing.T) {
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
