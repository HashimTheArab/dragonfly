package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

func TestPlacementObstructed_TypeAndTouchingBounds(t *testing.T) {
	for _, test := range []struct {
		name       string
		typeName   string
		bounds     cube.BBox
		obstructed bool
	}{
		{name: "player inside", typeName: "minecraft:player", bounds: cube.Box(0.1, 64, 0.1, 0.9, 65.8, 0.9), obstructed: true},
		{name: "custom inside", typeName: "custom:actor", bounds: cube.Box(0.1, 64, 0.1, 0.9, 65.8, 0.9), obstructed: true},
		{name: "item inside", typeName: "minecraft:item", bounds: cube.Box(0.1, 64, 0.1, 0.9, 65.8, 0.9)},
		{name: "arrow inside", typeName: "minecraft:arrow", bounds: cube.Box(0.1, 64, 0.1, 0.9, 65.8, 0.9)},
		{name: "orb inside", typeName: "minecraft:xp_orb", bounds: cube.Box(0.1, 64, 0.1, 0.9, 65.8, 0.9)},
		{name: "standing on block", typeName: "minecraft:player", bounds: cube.Box(0.1, 65, 0.1, 0.9, 66.8, 0.9)},
		{name: "touching side", typeName: "minecraft:player", bounds: cube.Box(1, 64, 0.1, 1.8, 65.8, 0.9)},
		{name: "within tolerance", typeName: "minecraft:player", bounds: cube.Box(0.99999, 64, 0.1, 1.8, 65.8, 0.9)},
		{name: "past tolerance", typeName: "minecraft:player", bounds: cube.Box(0.999, 64, 0.1, 1.8, 65.8, 0.9), obstructed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			actor := world.PlacementActor{Type: test.typeName, Bounds: test.bounds}
			if got := world.PlacementObstructed(nil, cube.Pos{0, 64, 0}, Stone{}, actor); got != test.obstructed {
				t.Fatalf("obstructed = %v, want %v", got, test.obstructed)
			}
		})
	}
}

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
