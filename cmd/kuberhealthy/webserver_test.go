package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

// TestWebServerHandlers checks that JSON and root handlers return HTTP 200.
func TestWebServerHandlers(t *testing.T) {
	t.Parallel()
	mux := newServeMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	for _, p := range []string{"/", "/json"} {
		resp, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatalf("failed to GET %s: %v", p, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 for %s got %d", p, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// TestRootServesUserInterface ensures the root endpoint serves the HTML status page.
func TestRootServesUserInterface(t *testing.T) {
	t.Parallel()
	mux := newServeMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	for _, u := range []string{ts.URL, ts.URL + "/"} {
		resp, err := http.Get(u)
		if err != nil {
			t.Fatalf("failed to GET %s: %v", u, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 for %s got %d", u, resp.StatusCode)
		}
		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		if !strings.Contains(string(b), "<title>Kuberhealthy Status</title>") {
			t.Fatalf("expected status page HTML for %s", u)
		}
	}
}

// TestHealthzHandler ensures the /healthz endpoint reports OK.
func TestHealthzHandler(t *testing.T) {
	t.Parallel()
	orig := Globals.kubeClient
	Globals.kubeClient = fake.NewSimpleClientset()
	t.Cleanup(func() { Globals.kubeClient = orig })

	mux := newServeMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("failed to GET /healthz: %v", err)
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 got %d", resp.StatusCode)
	}
	if string(b) != "OK" {
		t.Fatalf("expected body OK got %q", string(b))
	}
}
