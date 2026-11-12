package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestHealthCheckSpecOmitsUnsetPodSpec checks that an HTTP check does not serialize an empty pod spec,
// which would fail the CRD's required containers field on updates.
func TestHealthCheckSpecOmitsUnsetPodSpec(t *testing.T) {
	spec := HealthCheckSpec{HTTP: &HTTPCheck{URL: "http://example.com"}}

	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if strings.Contains(string(b), "podSpec") {
		t.Fatalf("expected no podSpec in %s", b)
	}
}

// TestHealthCheckSpecIsBuiltIn checks which specs run in the controller.
func TestHealthCheckSpecIsBuiltIn(t *testing.T) {
	podCheck := HealthCheckSpec{}
	if podCheck.IsBuiltIn() {
		t.Fatalf("pod check reported as built-in")
	}

	httpCheck := HealthCheckSpec{HTTP: &HTTPCheck{URL: "http://example.com"}}
	if !httpCheck.IsBuiltIn() {
		t.Fatalf("http check not reported as built-in")
	}

	tcpCheck := HealthCheckSpec{TCP: &TCPCheck{Address: "db:5432"}}
	if !tcpCheck.IsBuiltIn() {
		t.Fatalf("tcp check not reported as built-in")
	}
}
