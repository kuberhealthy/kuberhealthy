package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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
