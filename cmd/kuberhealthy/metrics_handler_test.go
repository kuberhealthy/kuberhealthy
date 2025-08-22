package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kuberhealthycheckv2 "github.com/kuberhealthy/crds/api/v2"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/controller"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPrometheusMetricsEndpoint(t *testing.T) {
	s := runtime.NewScheme()
	if err := kuberhealthycheckv2.AddToScheme(s); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}
	fakeClient := fake.NewClientBuilder().WithScheme(s).Build()
	KHController = &controller.KuberhealthyCheckReconciler{Client: fakeClient}
	GlobalConfig = &Config{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			http.NotFound(w, r)
			return
		}
		if err := prometheusMetricsHandler(w, r); err != nil {
			t.Fatalf("handler error: %v", err)
		}
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("failed to GET metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed reading body: %v", err)
	}
	if !strings.Contains(string(body), "kuberhealthy_running") {
		t.Fatalf("unexpected metrics body: %s", string(body))
	}
}
