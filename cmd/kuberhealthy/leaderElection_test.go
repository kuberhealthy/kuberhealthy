package main

import (
	"context"
	"testing"
)

// TestRunLeaderElectorExitsWhenLeadershipIsLost verifies that the process exits when the elector returns
// while the context is still active.
func TestRunLeaderElectorExitsWhenLeadershipIsLost(t *testing.T) {
	exitCode := -1
	runLeaderElector(context.Background(), func(context.Context) {}, func(code int) { exitCode = code })
	if exitCode != 1 {
		t.Fatalf("expected the process to exit with code 1 after leadership loss, got %d", exitCode)
	}
}

// TestRunLeaderElectorDoesNotExitOnShutdown verifies that a normal shutdown does not exit through the leader election path.
func TestRunLeaderElectorDoesNotExitOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	exited := false
	runLeaderElector(ctx, func(c context.Context) {
		cancel()
		<-c.Done()
	}, func(int) { exited = true })
	if exited {
		t.Fatal("did not expect the process to exit when the context was cancelled")
	}
}
