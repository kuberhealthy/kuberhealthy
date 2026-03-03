package main

import (
	"testing"
)

func TestFormatURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "IPv4 address",
			input:    "10.96.42.5",
			expected: "http://10.96.42.5",
		},
		{
			name:     "IPv6 address",
			input:    "fd00:4:32::6c47",
			expected: "http://[fd00:4:32::6c47]",
		},
		{
			name:     "IPv6 loopback",
			input:    "::1",
			expected: "http://[::1]",
		},
		{
			name:     "IPv6 full address",
			input:    "2001:db8:85a3:0000:0000:8a2e:0370:7334",
			expected: "http://[2001:db8:85a3:0000:0000:8a2e:0370:7334]",
		},
		{
			name:     "hostname",
			input:    "my-service.default.svc.cluster.local",
			expected: "http://my-service.default.svc.cluster.local",
		},
		{
			name:     "already has scheme",
			input:    "http://10.96.42.5",
			expected: "http://10.96.42.5",
		},
		{
			name:     "IPv4 loopback",
			input:    "127.0.0.1",
			expected: "http://127.0.0.1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatURL(tc.input)
			if result != tc.expected {
				t.Errorf("formatURL(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}
