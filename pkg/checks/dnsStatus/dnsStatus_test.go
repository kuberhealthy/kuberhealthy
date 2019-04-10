package dnsStatus

import (
	"testing"
)

const kubeConfigFile = "~/.kube/config"

func TestDnsStatusChecker(t *testing.T) {
	c := New([]string{})
	err := c.doChecks()
	if err != nil {
		t.Fatal(err)
	}
	up, errors := c.CurrentStatus()
	t.Log("up:", up)
	t.Log("errors:", errors)
	err = c.Shutdown()
	if err != nil {
		t.Fatal(err)
	}
}
