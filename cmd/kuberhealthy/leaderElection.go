package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kuberhealthy/kuberhealthy/v3/internal/kuberhealthy"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// leaderElectionRunner coordinates leader callbacks for controller tasks.
type leaderElectionRunner struct {
	kh       *kuberhealthy.Kuberhealthy
	identity string
}

// onStartedLeading starts leader-only tasks when a lease is acquired.
func (r *leaderElectionRunner) onStartedLeading(ctx context.Context) {
	err := r.kh.StartLeaderTasks(ctx)
	if err != nil {
		log.Errorln("leader election: failed to start leader tasks:", err)
	}
}

// onStoppedLeading stops leader-only tasks when the lease is lost.
func (r *leaderElectionRunner) onStoppedLeading() {
	r.kh.StopLeaderTasks()
}

// onNewLeader logs leader transitions.
func (r *leaderElectionRunner) onNewLeader(identity string) {
	if identity == r.identity {
		return
	}
	log.Infoln("leader election: new leader elected:", identity)
}

// startLeaderElection initializes Kubernetes Lease-based leader election and runs it in the background.
func startLeaderElection(ctx context.Context, cfg *Config, kubeClient kubernetes.Interface, kh *kuberhealthy.Kuberhealthy) error {
	identity := os.Getenv("POD_NAME")
	if identity == "" {
		identity = GetMyHostname("kuberhealthy")
	}

	lockConfig := resourcelock.ResourceLockConfig{Identity: identity}
	lock, err := resourcelock.New(resourcelock.LeasesResourceLock, cfg.LeaderElectionNamespace, cfg.LeaderElectionName, kubeClient.CoreV1(), kubeClient.CoordinationV1(), lockConfig)
	if err != nil {
		return fmt.Errorf("failed to create leader election lock: %w", err)
	}

	runner := &leaderElectionRunner{kh: kh, identity: identity}
	callbacks := leaderelection.LeaderCallbacks{
		OnStartedLeading: runner.onStartedLeading,
		OnStoppedLeading: runner.onStoppedLeading,
		OnNewLeader:      runner.onNewLeader,
	}

	leaderConfig := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   cfg.LeaderElectionLeaseDuration,
		RenewDeadline:   cfg.LeaderElectionRenewDeadline,
		RetryPeriod:     cfg.LeaderElectionRetryPeriod,
		Callbacks:       callbacks,
		ReleaseOnCancel: true,
	}

	elector, err := leaderelection.NewLeaderElector(leaderConfig)
	if err != nil {
		return fmt.Errorf("failed to create leader elector: %w", err)
	}

	log.Infoln("leader election: starting lease", cfg.LeaderElectionNamespace, cfg.LeaderElectionName)
	go elector.Run(ctx)
	return nil
}
