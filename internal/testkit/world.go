// Package testkit contains deterministic world lifecycle and waiting primitives shared by Dragonfly's tests.
package testkit

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world"
)

// NewWorld builds a world from conf and closes it when the test ends, reporting cleanup errors on the same test.
func NewWorld(t testing.TB, conf world.Config) *world.World {
	t.Helper()
	w := conf.New()
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Errorf("close world: %v", err)
		}
	})
	return w
}
