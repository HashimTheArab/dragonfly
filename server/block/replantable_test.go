package block_test

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

func TestReplantable_FullyGrownAndSupport(t *testing.T) {
	pos := cube.Pos{1, 2, 3}
	wheat, carrot, potato, beetroot := block.WheatSeeds{}, block.Carrot{}, block.Potato{}, block.BeetrootSeeds{}
	wheat.Growth, carrot.Growth, potato.Growth, beetroot.Growth = 7, 7, 7, 7
	below := func(face cube.Face) func(cube.Pos) (cube.Pos, cube.Face) {
		return func(p cube.Pos) (cube.Pos, cube.Face) { return p.Side(cube.FaceDown), face }
	}
	tests := []struct {
		name    string
		grown   block.Replantable
		young   block.Replantable
		support func(cube.Pos) (cube.Pos, cube.Face)
		holds   world.Block
		rejects world.Block
	}{
		{"wheat", wheat, block.WheatSeeds{}, below(cube.FaceUp), block.Farmland{}, block.Dirt{}},
		{"carrot", carrot, block.Carrot{}, below(cube.FaceUp), block.Farmland{}, block.Dirt{}},
		{"potato", potato, block.Potato{}, below(cube.FaceUp), block.Farmland{}, block.Dirt{}},
		{"beetroot", beetroot, block.BeetrootSeeds{}, below(cube.FaceUp), block.Farmland{}, block.Dirt{}},
		{"nether wart", block.NetherWart{Age: 3}, block.NetherWart{Age: 2}, below(cube.FaceUp), block.SoulSand{}, block.Farmland{}},
		{
			"cocoa", block.CocoaBean{Facing: cube.East, Age: 2}, block.CocoaBean{Facing: cube.East, Age: 1},
			func(p cube.Pos) (cube.Pos, cube.Face) { return p.Side(cube.FaceEast), cube.FaceWest },
			block.Log{Wood: block.JungleWood()}, block.Log{Wood: block.OakWood()},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !test.grown.FullyGrown() || test.young.FullyGrown() {
				t.Fatalf("FullyGrown() = %v for grown, %v for young; want true, false", test.grown.FullyGrown(), test.young.FullyGrown())
			}
			gotPos, gotFace := test.grown.Support(pos)
			wantPos, wantFace := test.support(pos)
			if gotPos != wantPos || gotFace != wantFace {
				t.Fatalf("Support() = %v %v, want %v %v", gotPos, gotFace, wantPos, wantFace)
			}
			if !test.grown.SupportedBy(test.holds) || test.grown.SupportedBy(test.rejects) {
				t.Fatalf("SupportedBy(%T) = %v, SupportedBy(%T) = %v; want true, false",
					test.holds, test.grown.SupportedBy(test.holds), test.rejects, test.grown.SupportedBy(test.rejects))
			}
		})
	}
}

// TestReplantable_StemsAreNotReplantable guards the one crop family whose
// harvest is not the plant itself.
func TestReplantable_StemsAreNotReplantable(t *testing.T) {
	for _, stem := range []world.Block{block.MelonSeeds{}, block.PumpkinSeeds{}} {
		if _, ok := stem.(block.Replantable); ok {
			t.Fatalf("%T implements Replantable; stems are harvested through their fruit", stem)
		}
	}
}
