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

		switch err {
		case nil:
			if dc.Hostname != "google.com" {
				t.Fatalf("doChecks failed to validate hostname correctly. Hostname: %s, Expected Check Result: %v", arg, expectedValue)
			}
			t.Logf("doChecks correctly validated hostname. ")
		default:
			if err.Error() != expectedValue.Error() {
				t.Fatalf("doChecks failed to validate hostname correctly. Hostname: %s, Expected Check Result: %v", arg, expectedValue)
			}
			t.Logf("doChecks correctly validated hostname. ")
		}
	}
}
