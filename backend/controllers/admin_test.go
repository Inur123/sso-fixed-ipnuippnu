package controllers

import "testing"

func TestParsePositiveQueryInt(t *testing.T) {
	tests := []struct {
		raw      string
		fallback int
		max      int
		want     int
		wantErr  bool
	}{
		{raw: "", fallback: 20, max: 100, want: 20},
		{raw: "1", fallback: 20, max: 100, want: 1},
		{raw: "100", fallback: 20, max: 100, want: 100},
		{raw: "0", fallback: 20, max: 100, wantErr: true},
		{raw: "101", fallback: 20, max: 100, wantErr: true},
		{raw: "abc", fallback: 20, max: 100, wantErr: true},
	}
	for _, test := range tests {
		got, err := parsePositiveQueryInt(test.raw, test.fallback, test.max)
		if got != test.want || (err != nil) != test.wantErr {
			t.Fatalf("raw=%q got=%d err=%v", test.raw, got, err)
		}
	}
}
