package session

import (
	"math"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func attachedFirework(owner *world.EntityHandle, lifetime time.Duration) *entity.FireworkBehaviour {
	return entity.FireworkBehaviourConfig{
		Firework:          item.Firework{Duration: time.Second},
		Owner:             owner,
		ExistenceDuration: lifetime,
		Attached:          true,
	}.New()
}

// Only the owner's client gets the boost, lasting the rocket's remaining lifetime rounded up to whole ticks.
func TestFireworkGlideBoostGoesToOwnerForRemainingLifetime(t *testing.T) {
	t.Parallel()
	owner := world.EntitySpawnOpts{}.New(entity.FireworkType, entity.FireworkBehaviourConfig{})
	other := world.EntitySpawnOpts{}.New(entity.FireworkType, entity.FireworkBehaviourConfig{})
	rocket := attachedFirework(owner, 1100*time.Millisecond)

	pk, until, ok := fireworkGlideBoost(rocket, 0, owner, 42)
	want := packet.MovementEffect{
		EntityRuntimeID: selfEntityRuntimeID,
		Type:            packet.MovementEffectTypeGlideBoost,
		Duration:        22,
		Tick:            42,
	}
	if !ok || *pk != want || until != 64 {
		t.Fatalf("fresh rocket boost = %+v until %d, %v; want %+v until 64", pk, until, ok, want)
	}
	for _, c := range []struct {
		lifetime, age time.Duration
		ticks         int32
	}{
		{1101 * time.Millisecond, 0, 23},
		{1101 * time.Millisecond, 1050 * time.Millisecond, 2},
		{1100 * time.Millisecond, 2 * time.Second, 0},
	} {
		if pk, _, _ := fireworkGlideBoost(attachedFirework(owner, c.lifetime), c.age, owner, 0); pk.Duration != c.ticks {
			t.Fatalf("lifetime %v at age %v boosts %d ticks, want %d", c.lifetime, c.age, pk.Duration, c.ticks)
		}
	}
	if pk, until, _ := fireworkGlideBoost(attachedFirework(owner, 0), 0, owner, 42); pk.Duration != -1 || until != math.MaxUint64 {
		t.Fatalf("never-expiring rocket boost = %d until %d, want -1 until max", pk.Duration, until)
	}
	if _, _, ok := fireworkGlideBoost(rocket, 0, other, 42); ok {
		t.Fatal("another player's client received the owner's boost")
	}
	loose := entity.FireworkBehaviourConfig{Owner: owner, ExistenceDuration: time.Second}.New()
	if _, _, ok := fireworkGlideBoost(loose, 0, owner, 42); ok {
		t.Fatal("an unattached firework boosted its owner")
	}
}

// The client replaces its boost with each one it receives, so a boost ending sooner is never sent.
func TestExtendGlideBoostNeverShortensTheClientBoost(t *testing.T) {
	t.Parallel()
	s := &Session{}
	if !s.extendGlideBoost(130) {
		t.Fatal("the first boost was not sent")
	}
	for _, until := range []uint64{120, 130} {
		if s.extendGlideBoost(until) {
			t.Fatalf("a boost ending on %d replaced one ending on 130", until)
		}
	}
	if !s.extendGlideBoost(140) || !s.extendGlideBoost(math.MaxUint64) || s.extendGlideBoost(200) {
		t.Fatal("a longer boost was dropped or an unlimited one was shortened")
	}
}
