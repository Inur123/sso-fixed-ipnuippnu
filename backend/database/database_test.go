package database

import "testing"

func TestValidProductionSSLMode(t *testing.T) {
	tests := []struct {
		name string
		host string
		mode string
		want bool
	}{
		{name: "verified remote", host: "db.example.com", mode: "verify-full", want: true},
		{name: "verified ca remote", host: "db.example.com", mode: "verify-ca", want: true},
		{name: "loopback ipv4", host: "127.0.0.1", mode: "disable", want: true},
		{name: "loopback hostname", host: "localhost", mode: "disable", want: true},
		{name: "loopback ipv6", host: "::1", mode: "disable", want: true},
		{name: "remote disable rejected", host: "10.0.0.4", mode: "disable", want: false},
		{name: "weak remote mode rejected", host: "db.example.com", mode: "require", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validProductionSSLMode(tt.host, tt.mode); got != tt.want {
				t.Fatalf("validProductionSSLMode(%q, %q) = %t, want %t", tt.host, tt.mode, got, tt.want)
			}
		})
	}
}
