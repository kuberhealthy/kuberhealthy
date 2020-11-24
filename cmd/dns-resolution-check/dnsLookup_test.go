package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestDnsLookup(t *testing.T) {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address2 string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, "udp", "8.8.8.8:53")
		},
	}
	testCase := make(map[string]error)
	testCase["bad.host.com"] = errors.New("DNS Status check determined that bad.host.com is DOWN")
	testCase["google.com"] = nil

	for arg, expectedValue := range testCase {
		host := arg

		err := dnsLookup(r, host)
		switch err {
		case nil:
			if host != "google.com" {
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
