package main

import (
	"errors"
	"testing"
)

func TestDoChecks(t *testing.T) {
	dc := New()
	testCase := make(map[string]error)
	testCase["bad.host.com"] = errors.New("DNS Status check determined that bad.host.com is DOWN: lookup bad.host.com: no such host")
	testCase["google.com"] = nil

	for arg, expectedValue := range testCase {
		dc.Hostname = arg

		err := dc.doChecks()

		if err == nil && dc.Hostname == "google.com" {
			t.Logf("doChecks correctly validated hostname. ")
			return
		}

		if err.Error() != expectedValue.Error() {
			t.Fatalf("doChecks failed to validate hostname correctly. Hostname: %s, Expected Check Result: %s", arg, expectedValue)
		}
		t.Logf("doChecks correctly validated hostname. ")
	}
}
