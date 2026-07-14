package testkit

import "github.com/df-mc/dragonfly/server/world"

// AdvanceUntil drives a synchronous world one tick at a time and checks ready after each tick. It returns false if
// ready is still false after maxTicks.
func AdvanceUntil(w *world.World, maxTicks int, ready func() bool) bool {
	for range maxTicks {
		w.AdvanceTick()
		if ready() {
			return true
		}
	}
	return false
}
