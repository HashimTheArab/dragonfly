package player

import (
	"fmt"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func TestAttackEntityDamagesHeldItemWhenAttackingNonLivingDamageableEntity(t *testing.T) {
	w := world.Config{Entities: world.EntityRegistryConfig{}.New([]world.EntityType{Type, testDamageableEntityType{}})}.New()
	defer w.Close()

	var attacked bool
	var before, after int
	<-w.Exec(func(tx *world.Tx) {
		p := tx.AddEntity(world.EntitySpawnOpts{Position: mgl64.Vec3{0, 1, 0}}.New(Type, Config{Name: "player"})).(*Player)
		sword := item.NewStack(item.Sword{Tier: item.ToolTierWood}, 1)
		p.SetHeldItems(sword, item.Stack{})
		before = sword.Durability()

		target := tx.AddEntity(world.EntitySpawnOpts{Position: mgl64.Vec3{3, 1, 0}}.New(testDamageableEntityType{}, testDamageableBehaviour{}))
		attacked = p.AttackEntity(target)

		held, _ := p.HeldItems()
		after = held.Durability()
	})

	if !attacked {
		t.Fatal("expected player to attack End crystal")
	}
	if after != before-1 {
		t.Fatalf("expected held sword durability to decrease by 1, before=%v after=%v", before, after)
	}
}

func TestTotemEffectsMatchJavaEdition(t *testing.T) {
	w := world.Config{Entities: world.EntityRegistryConfig{}.New([]world.EntityType{Type})}.New()
	defer w.Close()

	var testErr error
	<-w.Exec(func(tx *world.Tx) {
		p := tx.AddEntity(world.EntitySpawnOpts{Position: mgl64.Vec3{0, 1, 0}}.New(Type, Config{Name: "player", Health: 10, MaxHealth: 20})).(*Player)
		p.AddEffect(effect.New(effect.Speed, 1, time.Minute))

		p.applyTotemEffects()

		if got := p.Health(); got != 1 {
			testErr = fmt.Errorf("health after totem = %v, want 1", got)
			return
		}
		if _, ok := p.Effect(effect.Speed); ok {
			testErr = fmt.Errorf("totem should clear existing effects before applying Java effects")
			return
		}
		if err := assertEffect(p, effect.Regeneration, 2, 45*time.Second); err != nil {
			testErr = err
			return
		}
		if err := assertEffect(p, effect.FireResistance, 1, 40*time.Second); err != nil {
			testErr = err
			return
		}
		if err := assertEffect(p, effect.Absorption, 2, 5*time.Second); err != nil {
			testErr = err
			return
		}
		if got := p.Absorption(); got != 8 {
			testErr = fmt.Errorf("absorption after totem = %v, want 8", got)
			return
		}
	})
	if testErr != nil {
		t.Fatal(testErr)
	}
}

func assertEffect(p *Player, typ effect.Type, level int, duration time.Duration) error {
	eff, ok := p.Effect(typ)
	if !ok {
		return fmt.Errorf("missing effect %T", typ)
	}
	if eff.Level() != level {
		return fmt.Errorf("%T level = %d, want %d", typ, eff.Level(), level)
	}
	if eff.Duration() != duration {
		return fmt.Errorf("%T duration = %s, want %s", typ, eff.Duration(), duration)
	}
	return nil
}

type testDamageableEntityType struct{}

func (testDamageableEntityType) Open(tx *world.Tx, h *world.EntityHandle, data *world.EntityData) world.Entity {
	return entity.Open(tx, h, data)
}

func (testDamageableEntityType) EncodeEntity() string { return "dragonfly:test_damageable" }
func (testDamageableEntityType) BBox(world.Entity) cube.BBox {
	return cube.Box(-0.3, 0, -0.3, 0.3, 1.8, 0.3)
}
func (testDamageableEntityType) DecodeNBT(map[string]any, *world.EntityData) {}
func (testDamageableEntityType) EncodeNBT(*world.EntityData) map[string]any  { return nil }

type testDamageableBehaviour struct{}

func (b testDamageableBehaviour) Apply(data *world.EntityData)               { data.Data = b }
func (testDamageableBehaviour) Tick(*entity.Ent, *world.Tx) *entity.Movement { return nil }
func (testDamageableBehaviour) Hurt(*entity.Ent, float64, world.DamageSource) (float64, bool) {
	return 1, true
}
func (testDamageableBehaviour) Explode(*entity.Ent, mgl64.Vec3, float64, block.ExplosionConfig) {}
