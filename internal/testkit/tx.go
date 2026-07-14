package testkit

import (
	"context"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/world"
)

const worldTaskTimeout = 5 * time.Second

// Do executes f through w's transaction queue and waits up to five seconds for it to finish. Execution failures and
// timeouts stop the current test at the call site.
func Do(t testing.TB, w *world.World, f func(*world.Tx)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), worldTaskTimeout)
	defer cancel()
	if err := w.Do(f).Wait(ctx); err != nil {
		t.Fatalf("world task failed: %v", err)
	}
}
