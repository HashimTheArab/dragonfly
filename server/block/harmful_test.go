package block_test

import (
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// TestHarmfulMatchesContactBehaviour runs every registered block's step-on and
// inside callbacks against a living entity and requires HarmsOnContact to report
// exactly the states that hurt it, so a new hazard cannot go unreported. Sweet
// berry bushes hurt on a 1 in 20 roll, so each state gets up to 1000 contacts.
func TestHarmfulMatchesContactBehaviour(t *testing.T) {
	w := world.Config{Synchronous: true, Entities: entity.DefaultRegistry}.New()
	defer w.Close()

	pos := cube.Pos{0, 0, 0}
	w.Do(func(tx *world.Tx) {
		for _, b := range world.Blocks() {
			e := &contactEntity{}
			stepper, steps := b.(block.EntityStepper)
			insider, inside := b.(block.EntityInsider)
			for range 1000 {
				if e.harmed || !steps && !inside {
					break
				}
				if steps {
					stepper.EntityStepOn(pos, tx, e)
				}
				if inside {
					insider.EntityInside(pos, tx, e)
				}
			}
			h, ok := b.(block.Harmful)
			if reported := ok && h.HarmsOnContact(); reported != e.harmed {
				name, properties := b.EncodeBlock()
				t.Errorf("%s %v: HarmsOnContact = %v, contact harmed = %v", name, properties, reported, e.harmed)
			}
		}
	})
}

type contactEntity struct{ harmed bool }

func (*contactEntity) Close() error                  { return nil }
func (*contactEntity) H() *world.EntityHandle        { return nil }
func (*contactEntity) Position() mgl64.Vec3          { return mgl64.Vec3{} }
func (*contactEntity) Rotation() cube.Rotation       { return cube.Rotation{} }
func (*contactEntity) Velocity() mgl64.Vec3          { return mgl64.Vec3{0.2, 0, 0.2} }
func (*contactEntity) SetVelocity(mgl64.Vec3)        {}
func (*contactEntity) OnFireDuration() time.Duration { return 0 }
func (*contactEntity) SetOnFire(time.Duration)       {}
func (*contactEntity) Extinguish()                   {}
func (*contactEntity) ResetFallDistance()            {}
func (*contactEntity) FallDistance() float64         { return 0 }

func (e *contactEntity) Hurt(damage float64, _ world.DamageSource) (float64, bool) {
	e.harmed = true
	return damage, true
}

func (e *contactEntity) AddEffect(effect.Effect) { e.harmed = true }
