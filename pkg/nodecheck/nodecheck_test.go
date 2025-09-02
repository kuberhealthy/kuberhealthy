package nodeCheck

import (
	"context"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

func init() {
	// tests always run with debug logging
	log.SetLevel(log.DebugLevel)
}

// TestWaitForKuberhealthyEndpointReady verifies the helper returns an error when the endpoint cannot be reached.
func TestWaitForKuberhealthyEndpointReady(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	khEndpoint := "http://127.0.0.1:65535/"
	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	err := <-waitForKuberhealthyEndpointReady(ctx, khEndpoint)
	if err == nil {
		t.Error("Negative test failed for waitForKuberhealthyEndpointReady")
	}
}
