package entity

import (
	"math"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/item/potion"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func TestAnimalExplosionUsesSignedVerticalDirection(t *testing.T) {
	for _, test := range []struct {
		name   string
		source mgl64.Vec3
		wantY  float64
	}{
		{"above", mgl64.Vec3{0, 11, 0}, -.2},
		{"below", mgl64.Vec3{0, 9, 0}, .2},
		{"same height", mgl64.Vec3{1, 10, 0}, 0},
		{"diagonal", mgl64.Vec3{1, 11, 0}, -.2 / math.Sqrt2},
		{"same position", mgl64.Vec3{0, 10, 0}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := world.Config{Synchronous: true, Entities: DefaultRegistry}.New()
			defer w.Close()
			w.Do(func(tx *world.Tx) {
				a := tx.AddEntity(NewCow(world.EntitySpawnOpts{Position: mgl64.Vec3{0, 10, 0}})).(*Animal)
				source := tx.AddEntity(NewCow(world.EntitySpawnOpts{Position: test.source}))
				a.Explode(world.EntityExplosionSource{Entity: source, ExplosionSize: 1}, .2)
				if got := a.Velocity()[1]; math.IsNaN(got) || math.Abs(got-test.wantY) > 1e-9 {
					t.Fatalf("vertical explosion velocity = %v, want %v", got, test.wantY)
				}
				if a.Health() == a.MaxHealth() {
					t.Fatal("explosion stopped applying damage")
				}
			})
		})
	}
}

func TestAnimalMovementEffectsChangeVerticalMotion(t *testing.T) {
	for _, test := range []struct {
		name    string
		initial float64
		effects []effect.Effect
		wantY   float64
	}{
		{"levitation I", 0, []effect.Effect{effect.New(effect.Levitation, 1, time.Minute)}, .01},
		{"levitation III", 0, []effect.Effect{effect.New(effect.Levitation, 3, time.Minute)}, .03},
		{"slow falling descending", -.2, []effect.Effect{effect.New(effect.SlowFalling, 1, time.Minute)}, -.2058},
		{"slow falling ascending", .2, []effect.Effect{effect.New(effect.SlowFalling, 1, time.Minute)}, .1176},
		{"levitation takes precedence", 0, []effect.Effect{effect.New(effect.Levitation, 1, time.Minute), effect.New(effect.SlowFalling, 1, time.Minute)}, .01},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := world.Config{Synchronous: true, Entities: DefaultRegistry}.New()
			defer w.Close()
			w.Do(func(tx *world.Tx) {
				a := tx.AddEntity(NewCow(world.EntitySpawnOpts{Position: mgl64.Vec3{0, 20, 0}, Velocity: mgl64.Vec3{0, test.initial, 0}})).(*Animal)
				for _, eff := range test.effects {
					a.AddEffect(eff)
				}
				a.Tick(tx, 1)
				if got := a.Velocity()[1]; math.Abs(got-test.wantY) > 1e-9 {
					t.Fatalf("effect vertical velocity = %v, want %v", got, test.wantY)
				}
			})
		})
	}
}

func TestAnimalMovementEffectExpiryRestoresGravity(t *testing.T) {
	for name, typ := range map[string]effect.LastingType{"levitation": effect.Levitation, "slow falling": effect.SlowFalling} {
		t.Run(name, func(t *testing.T) {
			w := world.Config{Synchronous: true, Entities: DefaultRegistry}.New()
			defer w.Close()
			w.Do(func(tx *world.Tx) {
				a := tx.AddEntity(NewCow(world.EntitySpawnOpts{Position: mgl64.Vec3{0, 20, 0}, Velocity: mgl64.Vec3{0, -.2, 0}})).(*Animal)
				a.AddEffect(effect.New(typ, 1, time.Second/20))
				a.Tick(tx, 1)
				before := a.Velocity()[1]
				a.Tick(tx, 2)
				if got, want := a.Velocity()[1], (before-.08)*.98; math.Abs(got-want) > 1e-9 {
					t.Fatalf("expired effect vertical velocity = %v, want %v", got, want)
				}
			})
		})
	}
}

func TestAnimalLevitationStillCollidesWithCeilings(t *testing.T) {
	w := world.Config{Synchronous: true, Entities: DefaultRegistry}.New()
	defer w.Close()
	w.Do(func(tx *world.Tx) {
		tx.SetBlock(cube.Pos{0, 22, 0}, block.Stone{}, nil)
		a := tx.AddEntity(NewCow(world.EntitySpawnOpts{Position: mgl64.Vec3{.5, 20, .5}, Velocity: mgl64.Vec3{0, 1, 0}})).(*Animal)
		a.AddEffect(effect.New(effect.Levitation, 1, time.Minute))
		a.Tick(tx, 1)
		if a.Position()[1]+CowType.BBox(a).Height() > 22 || a.Velocity()[1] != 0 {
			t.Fatalf("levitating cow crossed ceiling: position=%v, velocity=%v", a.Position(), a.Velocity())
		}
	})
}

func TestAnimalSplashWaterExtinguishesBurning(t *testing.T) {
	w := world.Config{Synchronous: true, Entities: DefaultRegistry}.New()
	defer w.Close()
	w.Do(func(tx *world.Tx) {
		a := tx.AddEntity(NewCow(world.EntitySpawnOpts{Position: mgl64.Vec3{0, 10, 0}})).(*Animal)
		a.SetOnFire(5 * time.Second)
		a.state().fireElapsed = 750 * time.Millisecond
		bottle := tx.AddEntity(NewSplashPotion(world.EntitySpawnOpts{Position: a.Position()}, potion.Water(), a)).(*Ent)
		potionSplash(1, potion.Water(), false)(bottle, tx, nil)
		if a.OnFireDuration() != 0 || a.state().fireElapsed != 0 {
			t.Fatalf("splash water left burn duration %v and progress %v", a.OnFireDuration(), a.state().fireElapsed)
		}
	})
}
