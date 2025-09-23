package health

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

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
	err := s.WriteHTTPStatusResponse(rr)
	if err != nil {
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
