package session

import (
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func rocketHandle() *world.EntityHandle {
	return world.EntitySpawnOpts{}.New(entity.FireworkType, entity.FireworkBehaviourConfig{})
}

// A rocket's boost lasts its lifetime in whole ticks from the owner's input tick, decided once per rocket.
func TestGlideBoostLastsTheRocketLifetimeAndIsDecidedOncePerRocket(t *testing.T) {
	t.Parallel()
	var b glideBoost
	rocket := rocketHandle()
	effect, ok := b.decide(rocket, 1100*time.Millisecond, 42)
	want := packet.MovementEffect{Type: packet.MovementEffectTypeGlideBoost, Duration: 22, Tick: 42}
	if !ok || effect != want {
		t.Fatalf("first rocket boost = %+v, %v; want %+v", effect, ok, want)
	}
	if again, ok := b.decide(rocket, 0, 60); !ok || again != want {
		t.Fatalf("another viewer of the same rocket got %+v, %v; want %+v", again, ok, want)
	}
	if effect, _ := b.decide(rocketHandle(), 1101*time.Millisecond, 100); effect.Duration != 23 {
		t.Fatalf("a 1101ms rocket boosts %d ticks, want 23", effect.Duration)
	}
}

// A boost replaces the current one only when it lasts at least as long as what remains, unlimited ones included.
func TestGlideBoostReplacesOnlyWhenItLastsAtLeastAsLong(t *testing.T) {
	t.Parallel()
	var b glideBoost
	steps := []struct {
		lifetime time.Duration
		tick     uint64
		sent     bool
	}{
		{time.Second, 100, true},             // ends on 120
		{500 * time.Millisecond, 105, false}, // ends on 115
		{750 * time.Millisecond, 105, true},  // ends on 120, a tie
		{50 * time.Millisecond, 125, true},   // the current boost has run out
		{0, 130, true},                       // never ends
		{time.Hour, 140, false},
		{0, 150, true},
	}
	for i, step := range steps {
		if _, sent := b.decide(rocketHandle(), step.lifetime, step.tick); sent != step.sent {
			t.Fatalf("step %d: sent = %v, want %v", i, sent, step.sent)
		}
	}
	if b.effect.Duration != -1 {
		t.Fatalf("a never-expiring rocket boosts for %d ticks, want -1", b.effect.Duration)
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
