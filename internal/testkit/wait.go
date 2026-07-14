package testkit

import (
	"testing"
	"time"
)

const pollInterval = 10 * time.Millisecond

// Eventually polls ready every ten milliseconds until it returns true. If timeout expires first, the current test
// fails at the call site.
func Eventually(t testing.TB, timeout time.Duration, ready func() bool) {
	t.Helper()
	if ready() {
		return
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if ready() {
				return
			}
		case <-deadline.C:
			t.Fatalf("condition was not reached within %s", timeout)
		}
	}
}
