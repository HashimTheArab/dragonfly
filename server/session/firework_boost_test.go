package session

import (
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// A viewer writes the boost it was given for the owner, with the owner's input tick; a negative one never ends.
func TestGlideBoostEffectCarriesTheDecidedBoost(t *testing.T) {
	t.Parallel()
	want := packet.MovementEffect{EntityRuntimeID: 7, Type: packet.MovementEffectTypeGlideBoost, Duration: 23, Tick: 42}
	if got := glideBoostEffect(7, 23*time.Second/20, 42); *got != want {
		t.Fatalf("boost effect = %+v, want %+v", *got, want)
	}
	if got := glideBoostEffect(1, -time.Second/20, 42); got.Duration != -1 {
		t.Fatalf("an unending boost lasts %d ticks, want -1", got.Duration)
	}
}

// Rockets fly their Duration plus a whole number of ticks from 0 to 11.
func TestFireworkRandomisedDurationAddsWholeTicks(t *testing.T) {
	t.Parallel()
	const tick = time.Second / 20
	f := item.Firework{Duration: time.Second}
	seen := map[time.Duration]bool{}
	for range 2000 {
		extra := f.RandomisedDuration() - f.Duration
		if extra < 0 || extra > 11*tick || extra%tick != 0 {
			t.Fatalf("randomised extra flight %v is not 0-11 whole ticks", extra)
		}
		seen[extra] = true
	}
	if len(seen) != 12 {
		t.Fatalf("saw %d distinct extra flight lengths, want 12", len(seen))
	}
}
