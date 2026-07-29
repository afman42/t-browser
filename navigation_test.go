package main

import (
	"testing"
)

func TestIsInternalAddress(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		// localhost
		{"localhost", true},
		{"LOCALHOST", true},

		// 127.* loopback
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"127.255.255.255", true},

		// 10.* private
		{"10.0.0.1", true},
		{"10.255.255.255", true},

		// 192.168.* private
		{"192.168.0.1", true},
		{"192.168.1.100", true},

		// 172.16-31 private (BUG FIX: old code allowed 172.32-39)
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		// 172.32+ should NOT be blocked (not private)
		{"172.32.0.1", false},
		{"172.33.0.1", false},
		{"172.15.0.1", false},
		// Old bug: host[4] check allowed 172.32-39 via single-digit check
		{"172.20.0.1", true},
		{"172.25.0.1", true},

		// 169.254.* link-local (NEW: was missing)
		{"169.254.0.1", true},
		{"169.254.169.254", true},

		// 0.0.0.0 (NEW: was missing)
		{"0.0.0.0", true},

		// IPv6 loopback (NEW: was missing)
		{"::1", true},
		{"[::1]", true},

		// IPv6 unspecified (NEW)
		{"::", true},
		{"[::]", true},

		// Public addresses — should NOT be blocked
		{"example.com", false},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"172.32.0.1", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			got := isInternalAddress(tc.host)
			if got != tc.want {
				t.Errorf("isInternalAddress(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

func TestValidateAndSanitizeURLSSRFProtections(t *testing.T) {
	b := &Browser{}
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"IPv6 loopback", "http://[::1]:8080/", true},
		{"link-local 169.254", "http://169.254.169.254/latest/meta-data/", true},
		{"0.0.0.0", "http://0.0.0.0/", true},
		{"172.32 not private", "http://172.32.0.1/", false},
		{"172.16 private", "http://172.16.0.1/", true},
		{"public domain", "https://example.com/", false},
		{"public IP", "http://8.8.8.8/", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := b.validateAndSanitizeURL(tc.url)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateAndSanitizeURL(%q) error = %v, wantErr = %v", tc.url, err, tc.wantErr)
			}
		})
	}
}
