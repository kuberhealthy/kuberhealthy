package khcheck

import (
	"context"
	"sync"
	"time"
)

// KHCheck is used by Kuberhealthy internally to manage the scheduling and status of checks
type KHCheck struct {
	sync.WaitGroup
	CheckName   string // the name of this checker
	Namespace   string
	RunInterval time.Duration // how often this check runs a loop
	RunTimeout  time.Duration // time check must run completely within
	// OriginalPodSpec          apiv1.PodSpec // the user-provided spec of the pod
	KuberhealthyReportingURL string // the URL that the check should want to report results back to
	ExtraAnnotations         map[string]string
	ExtraLabels              map[string]string
	currentCheckUUID         string             // the UUID of the current external checker running
	shutdownCTXFunc          context.CancelFunc // used to cancel things in-flight when shutting down gracefully
	shutdownCTX              context.Context    // a context used for shutting down the check gracefully
}
