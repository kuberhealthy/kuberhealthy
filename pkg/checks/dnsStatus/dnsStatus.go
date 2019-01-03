// Package dnsStatus implements a DNS checker for Kuberhealthy
// It verifies that local DNS and external DNS are functioning correctly
package dnsStatus

import (
	"errors"
	"net"
	"time"

	// required for oidc kubectl testing
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"k8s.io/client-go/kubernetes"
)

// Checker validates that DNS is functioning correctly
type Checker struct {
	FailureTimeStamp map[string]time.Time
	Errors           []string
	MaxTimeInFailure float64 // TODO - make configurable
	client           *kubernetes.Clientset
	Endpoints        []string // TODO - Make configurable
}

// New returns a new Checker
func New(endpoints []string) *Checker {
	defaultEndpoints := []string{
		"kubernetes.default",
		"cloud.google.com",
		"aws.amazon.com",
	}
	if len(endpoints) == 0 {
		endpoints = defaultEndpoints
	}
	return &Checker{
		FailureTimeStamp: make(map[string]time.Time),
		MaxTimeInFailure: 300,
		Errors:           []string{},
		Endpoints:        endpoints,
	}
}

// Name returns the name of this checker
func (dc *Checker) Name() string {
	return "DnsStatusChecker"
}

// CheckNamespace returns the namespace of this checker
func (dc *Checker) CheckNamespace() string {
	return ""
}

// Interval returns the interval at which this check runs
func (dc *Checker) Interval() time.Duration {
	return time.Minute * 2
}

// Timeout returns the maximum run time for this check before it times out
func (dc *Checker) Timeout() time.Duration {
	return time.Minute * 1
}

// Shutdown is implemented to satisfy KuberhealthyCheck but is not used
func (dc *Checker) Shutdown() error {
	return nil
}

// CurrentStatus returns the status of the check as of right now
func (dc *Checker) CurrentStatus() (bool, []string) {
	if len(dc.Errors) > 0 {
		return false, dc.Errors
	}
	return true, dc.Errors
}

// clearErrors clears all errors
func (dc *Checker) clearErrors() {
	dc.Errors = []string{}
}

// Run implements the entrypoint for check execution
func (dc *Checker) Run(client *kubernetes.Clientset) error {
	doneChan := make(chan error)

	dc.client = client
	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := dc.doChecks()
		doneChan <- err
	}(doneChan)

	// wait for either a timeout or job completion
	select {
	case <-time.After(dc.Interval()):
		// The check has timed out because its time to run again
		// TODO - set check to failed because of timeout
		return errors.New("Failed to complete checks for " + dc.Name() + " in time!  Next run came up but check was still running.")
	case <-time.After(dc.Timeout()):
		// The check has timed out after its specified timeout period
		return errors.New("Failed to complete checks for " + dc.Name() + " in time!  Timeout was reached.")
	case err := <-doneChan:
		return err
	}
}

// doChecks does validations on DNS calls to various endpoints
func (dc *Checker) doChecks() error {
	dc.clearErrors()
	for _, address := range dc.Endpoints {
		_, err := net.LookupHost(address)
		if err != nil {
			dc.Errors = append(dc.Errors, err.Error())
		}
	}
	return nil
}
