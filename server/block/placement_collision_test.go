package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

func TestPlacementModel_ObstructedByTypeAndTouchingBounds(t *testing.T) {
	solid := placementTestEntityType{name: "minecraft:player"}
	passable := placementTestPassableType{placementTestEntityType{name: "minecraft:item"}}
	for _, test := range []struct {
		name       string
		typ        world.EntityType
		bounds     cube.BBox
		obstructed bool
	}{
		{name: "solid inside", typ: solid, bounds: cube.Box(0.1, 64, 0.1, 0.9, 65.8, 0.9), obstructed: true},
		{name: "unregistered inside", bounds: cube.Box(0.1, 64, 0.1, 0.9, 65.8, 0.9), obstructed: true},
		{name: "passable inside", typ: passable, bounds: cube.Box(0.1, 64, 0.1, 0.9, 65.8, 0.9)},
		{name: "standing on block", typ: solid, bounds: cube.Box(0.1, 65, 0.1, 0.9, 66.8, 0.9)},
		{name: "touching side", typ: solid, bounds: cube.Box(1, 64, 0.1, 1.8, 65.8, 0.9)},
		{name: "within tolerance", typ: solid, bounds: cube.Box(0.99999, 64, 0.1, 1.8, 65.8, 0.9)},
		{name: "past tolerance", typ: solid, bounds: cube.Box(0.999, 64, 0.1, 1.8, 65.8, 0.9), obstructed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := world.NewPlacementModel(nil, cube.Pos{0, 64, 0}, Stone{})
			if got := model.Obstructed(world.PlacementActor{Type: test.typ, Bounds: test.bounds}); got != test.obstructed {
				t.Fatalf("obstructed = %v, want %v", got, test.obstructed)
			}
		})
	}
}

// placementTestEntityType is a minimal world.EntityType for placement collision rules.
type placementTestEntityType struct{ name string }

func (placementTestEntityType) Open(*world.Tx, *world.EntityHandle, *world.EntityData) world.Entity {
	return nil
}
func (t placementTestEntityType) EncodeEntity() string                      { return t.name }
func (placementTestEntityType) BBox(world.Entity) cube.BBox                 { return cube.BBox{} }
func (placementTestEntityType) DecodeNBT(map[string]any, *world.EntityData) {}
func (placementTestEntityType) EncodeNBT(*world.EntityData) map[string]any  { return nil }

// placementTestPassableType is an entity type that never obstructs placement.
type placementTestPassableType struct{ placementTestEntityType }

func (placementTestPassableType) PassableByPlacement() bool { return true }

func TestCanReplace_UsesBlockRule(t *testing.T) {
	for _, test := range []struct {
		name    string
		current world.Block
		with    world.Block
		want    bool
	}{
		{name: "air", current: Air{}, with: Stone{}, want: true},
		{name: "plant", current: ShortGrass{}, with: Stone{}, want: true},
		{name: "solid", current: Stone{}, with: Stone{}},
		{name: "missing current", with: Stone{}},
		{name: "missing material", current: Air{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := world.CanReplace(test.current, test.with); got != test.want {
				t.Fatalf("can replace = %v, want %v", got, test.want)
			}
		})
	}
}
