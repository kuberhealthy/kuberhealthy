package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
)

func TestDoChecks(t *testing.T) {
	dc := New()
	client, err := kubeClient.Create(filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}
	dc.client = client
	testCase := make(map[string]error)
	testCase["bad.host.com"] = errors.New("DNS Status check determined that bad.host.com is DOWN")
	testCase["google.com"] = nil

	for arg, expectedValue := range testCase {
		dc.Hostname = arg

		err := dc.doChecks()
		if strings.Contains(err.Error(), "i/o timeout") {
			t.Logf("DNS Timed out")
			continue
		}
		switch err {
		case nil:
			if dc.Hostname != "google.com" {
				t.Fatalf("doChecks failed to validate hostname correctly. Hostname: %s, Expected Check Result: %v", arg, expectedValue)
			}
			t.Logf("doChecks correctly validated hostname. ")
		default:
			if !strings.Contains(err.Error(), expectedValue.Error()) {
				t.Fatalf("doChecks failed to validate hostname correctly. Hostname: %s, Expected Check Result: %v", arg, expectedValue)
			}
			t.Logf("doChecks correctly validated hostname. ")
		}
	}
}
