package mailqueue

import (
	"testing"
	"time"
)

func TestRetryDelayUsesBoundedExponentialBackoff(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: 2 * time.Second},
		{attempt: 1, want: 2 * time.Second},
		{attempt: 2, want: 4 * time.Second},
		{attempt: 8, want: 256 * time.Second},
		{attempt: 20, want: 256 * time.Second},
	}
	for _, test := range tests {
		if got := retryDelay(test.attempt); got != test.want {
			t.Fatalf("attempt=%d got=%s want=%s", test.attempt, got, test.want)
		}
	}
}

func TestOptionalEnvIntUsesDefaultsAndBounds(t *testing.T) {
	t.Setenv("MAIL_QUEUE_CONCURRENCY", "")
	if got, err := optionalEnvInt("MAIL_QUEUE_CONCURRENCY", 2, 1, 8); err != nil || got != 2 {
		t.Fatalf("got=%d err=%v", got, err)
	}
	t.Setenv("MAIL_QUEUE_CONCURRENCY", "9")
	if _, err := optionalEnvInt("MAIL_QUEUE_CONCURRENCY", 2, 1, 8); err == nil {
		t.Fatal("out-of-range value must fail")
	}
}
