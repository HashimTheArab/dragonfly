package item_test

import (
	"slices"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type crystalTerrain map[cube.Pos]world.Block

// Block supplies explicit fixture blocks, with air elsewhere.
func (w crystalTerrain) Block(pos cube.Pos) world.Block {
	if b, ok := w[pos]; ok {
		return b
	}
	return block.Air{}
}

func TestEndCrystal_CanPlace(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	for _, tt := range []struct {
		name   string
		base   world.Block
		above  world.Block
		second world.Block
		bounds cube.Range
		boxes  []cube.BBox
		want   bool
	}{
		{name: "obsidian", base: block.Obsidian{}, want: true},
		{name: "bedrock", base: block.Bedrock{}, want: true},
		{name: "crying obsidian", base: block.Obsidian{Crying: true}},
		{name: "stone", base: block.Stone{}},
		{name: "first air cell blocked", base: block.Obsidian{}, above: block.Stone{}},
		{name: "second air cell blocked", base: block.Obsidian{}, second: block.Stone{}},
		{name: "water", base: block.Obsidian{}, above: block.Water{Depth: 8}},
		{name: "adjacent opponent", base: block.Obsidian{}, boxes: []cube.BBox{cube.Box(-.8, 1, .2, -.2, 2.8, .8)}, want: true},
		{name: "occupied air", base: block.Obsidian{}, boxes: []cube.BBox{cube.Box(.2, 1, .2, .8, 2.8, .8)}},
		{name: "occupied base", base: block.Obsidian{}, boxes: []cube.BBox{cube.Box(.2, 0, .2, .8, .8, .8)}},
		{name: "touching edge", base: block.Obsidian{}, boxes: []cube.BBox{cube.Box(-1, 0, 0, 0, 2, 1)}, want: true},
		{name: "out of bounds air", base: block.Obsidian{}, bounds: cube.Range{-64, 1}},
		{name: "out of bounds base", base: block.Obsidian{}, bounds: cube.Range{1, 319}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bounds := tt.bounds
			if bounds == (cube.Range{}) {
				bounds = cube.Range{-64, 319}
			}
			w := crystalTerrain{{}: tt.base}
			if tt.above != nil {
				w[cube.Pos{0, 1, 0}] = tt.above
			}
			if tt.second != nil {
				w[cube.Pos{0, 2, 0}] = tt.second
			}
			if got := (item.EndCrystal{}).CanPlace(cube.Pos{}, w, bounds, slices.Values(tt.boxes)); got != tt.want {
				t.Fatalf("CanPlace = %v, want %v", got, tt.want)
			}
		})
	}
	if got := (item.EndCrystal{}).PlacementPosition(cube.Pos{-2, 4, 3}); got != (mgl64.Vec3{-1.5, 5, 3.5}) {
		t.Fatalf("spawn point = %v", got)
	}
}
