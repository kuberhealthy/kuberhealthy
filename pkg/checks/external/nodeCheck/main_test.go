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

func TestWaitForKuberhealthyEndpointReady(t *testing.T) {
	khEndpoint := "http://non.existent/"
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err := <-waitForKuberhealthyEndpointReady(ctx, khEndpoint)
	if err == nil {
		t.Error("Negative test failed for waitForKuberhealthyEndpointReady")
	}
}
