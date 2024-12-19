package nodeCheck

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

func init() {
	// tests always run with debug logging
	log.SetLevel(log.DebugLevel)
}

func TestWaitForKuberhealthyEndpointReady(t *testing.T) {
	// Test with a healthy target
	tsHealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello")
	}))
	defer tsHealthy.Close()
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err := <-waitForKuberhealthyEndpointReady(ctx, tsHealthy.URL)
	if err != nil {
		t.Error("Test failed for waitForKuberhealthyEndpointReady. Test target endpoint is not healthy. Err: ", err.Error())
	}

	// Test with a target not responding
	khEndpoint := "http://non.existent/"
	ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
	err = <-waitForKuberhealthyEndpointReady(ctx, khEndpoint)
	if err == nil {
		t.Error("Negative test failed for waitForKuberhealthyEndpointReady. Test target endpoint was supposed to be unreachable")
	}

	// Test with a target responding a http error
	tsInError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer tsInError.Close()
	ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
	err = <-waitForKuberhealthyEndpointReady(ctx, tsInError.URL)
	if err == nil {
		t.Error("Negative test failed for waitForKuberhealthyEndpointReady. Test target endpoint was supposed to be not ready")
	}
}
