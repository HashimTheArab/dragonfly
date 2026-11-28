package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// fallTestEntity records whether a block reset its fall distance. Calls to
// any other entity method panic, which the test treats as unknown behaviour.
type fallTestEntity struct {
	world.Entity
	reset bool
}

// ResetFallDistance records the reset.
func (e *fallTestEntity) ResetFallDistance() { e.reset = true }

// FallDistance reports no fall, so farmland is not trampled.
func (e *fallTestEntity) FallDistance() float64 { return 0 }

// Every registered block that changes the fall of an entity inside or landing
// on it declares the change through FallDistanceModifier, so a fall can be
// predicted without running the block's behaviour.
func TestFallDistanceModifiersAreDeclared(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	w := world.Config{Synchronous: true}.New()
	defer w.Close()
	pos := cube.Pos{0, 64, 0}
	const fallen = 10.0
	runWorld(w, func(tx *world.Tx) {
		for _, b := range world.Blocks() {
			name, _ := b.EncodeBlock()
			want := fallen
			if modifier, ok := b.(FallDistanceModifier); ok {
				want = modifier.ModifyFallDistance(fallen)
			}
			if insider, ok := b.(EntityInsider); ok {
				e := &fallTestEntity{}
				func() {
					defer func() { _ = recover() }()
					insider.EntityInside(pos, tx, e)
				}()
				if e.reset != (want == 0) {
					t.Errorf("%s (%T): EntityInside resets fall distance %v, ModifyFallDistance(%v) = %v", name, b, e.reset, fallen, want)
				}
			}
			if lander, ok := b.(EntityLander); ok {
				distance := fallen
				func() {
					defer func() { _ = recover() }()
					lander.EntityLand(pos, tx, &fallTestEntity{}, &distance)
				}()
				if distance != want {
					t.Errorf("%s (%T): EntityLand leaves fall distance %v, ModifyFallDistance(%v) = %v", name, b, distance, fallen, want)
				}
			}
		}
	})
}
