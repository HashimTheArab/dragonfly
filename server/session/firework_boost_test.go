package session

import (
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Only the owner's client gets the boost, lasting the rocket's remaining lifetime from its latest input tick.
func TestFireworkGlideBoostGoesToOwnerForRemainingLifetime(t *testing.T) {
	t.Parallel()
	owner := world.EntitySpawnOpts{}.New(entity.FireworkType, entity.FireworkBehaviourConfig{})
	other := world.EntitySpawnOpts{}.New(entity.FireworkType, entity.FireworkBehaviourConfig{})
	attached := entity.FireworkBehaviourConfig{
		Firework:          item.Firework{Duration: time.Second},
		Owner:             owner,
		ExistenceDuration: 1100 * time.Millisecond,
		Attached:          true,
	}.New()

	pk, ok := fireworkGlideBoost(attached, 0, owner, 42)
	want := packet.MovementEffect{
		EntityRuntimeID: selfEntityRuntimeID,
		Type:            packet.MovementEffectTypeGlideBoost,
		Duration:        22,
		Tick:            42,
	}
	if !ok || *pk != want {
		t.Fatalf("fresh rocket boost = %+v, %v; want %+v", pk, ok, want)
	}
	if pk, _ := fireworkGlideBoost(attached, 500*time.Millisecond, owner, 52); pk.Duration != 12 {
		t.Fatalf("half-spent rocket boost duration = %d, want 12", pk.Duration)
	}
	if pk, _ := fireworkGlideBoost(attached, 2*time.Second, owner, 80); pk.Duration != 0 {
		t.Fatalf("expired rocket boost duration = %d, want 0", pk.Duration)
	}
	if _, ok := fireworkGlideBoost(attached, 0, other, 42); ok {
		t.Fatal("another player's client received the owner's boost")
	}
	loose := entity.FireworkBehaviourConfig{Owner: owner, ExistenceDuration: time.Second}.New()
	if _, ok := fireworkGlideBoost(loose, 0, owner, 42); ok {
		t.Fatal("an unattached firework boosted its owner")
	}
}
