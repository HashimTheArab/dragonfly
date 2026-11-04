package block_test

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

// affectedBlocks is every block that names a speed for a sword or for shears.
var affectedBlocks = []world.Block{
	block.Cobweb{}, block.Leaves{}, block.Wool{}, block.Vines{}, block.Bamboo{},
	block.BambooSapling{}, block.MossCarpet{}, block.Pumpkin{}, block.Melon{}, block.CocoaBean{},
}

// TestSwordMiningSpeed pins the speed each block gives a sword. A sword has no tier speed of its own:
// cobweb and bamboo it cuts through, the plants it merely brushes aside, everything else it does not
// help with at all.
func TestSwordMiningSpeed(t *testing.T) {
	sword := item.Sword{Tier: item.ToolTierDiamond}
	tests := []struct {
		block world.Block
		want  float64
	}{
		{block.Cobweb{}, 15},
		{block.Bamboo{}, 30},
		{block.BambooSapling{}, 30},
		{block.Leaves{}, 1.5},
		{block.Vines{}, 1.5},
		{block.MossCarpet{}, 1.5},
		{block.Pumpkin{}, 1.5},
		{block.Pumpkin{Carved: true}, 1.5},
		{block.Melon{}, 1.5},
		{block.CocoaBean{}, 1.5},
		{block.Stone{}, 1},
		{block.Dirt{}, 1},
	}
	for _, tt := range tests {
		if got := sword.BaseMiningEfficiency(tt.block); got != tt.want {
			t.Errorf("%T: got %v, want %v", tt.block, got, tt.want)
		}
	}
}

// TestShearsMiningSpeed pins the three speeds Bedrock sorts blocks into for shears.
func TestShearsMiningSpeed(t *testing.T) {
	shears := item.Shears{}
	tests := []struct {
		block world.Block
		want  float64
	}{
		{block.Cobweb{}, 15},
		{block.Leaves{}, 15},
		{block.Wool{}, 5},
		{block.Vines{}, 2},
		{block.Stone{}, 1},
		{block.Pumpkin{}, 1},
	}
	for _, tt := range tests {
		if got := shears.BaseMiningEfficiency(tt.block); got != tt.want {
			t.Errorf("%T: got %v, want %v", tt.block, got, tt.want)
		}
	}
}

// TestToolSpecificSpeedsAreReachable guards the split between the two halves of this: a block naming a
// speed for a tool whose BreakInfo does not also call that tool effective would never use the speed,
// and a block calling one of them effective without naming a speed would mine at the bare-handed 1.
func TestToolSpecificSpeedsAreReachable(t *testing.T) {
	sword, shears := item.Sword{Tier: item.ToolTierDiamond}, item.Shears{}
	for _, b := range affectedBlocks {
		effective := b.(block.Breakable).BreakInfo().Effective
		_, namesSword := b.(item.SwordMineable)
		if namesSword != effective(sword) {
			t.Errorf("%T: names a sword speed %v, but Effective(sword) is %v", b, namesSword, effective(sword))
		}
		_, namesShears := b.(item.ShearsMineable)
		if namesShears != effective(shears) {
			t.Errorf("%T: names a shears speed %v, but Effective(shears) is %v", b, namesShears, effective(shears))
		}
	}
}

// TestToolSpecificBreakDurations checks the speeds reach BreakDuration, against times published for
// Bedrock Edition.
func TestToolSpecificBreakDurations(t *testing.T) {
	sword := item.NewStack(item.Sword{Tier: item.ToolTierDiamond}, 1)
	shears := item.NewStack(item.Shears{}, 1)

	tests := []struct {
		name      string
		block     world.Block
		stack     item.Stack
		wantTicks int64
	}{
		{name: "shears shear wool", block: block.Wool{}, stack: shears, wantTicks: 5},
		{name: "shears cut cobweb", block: block.Cobweb{}, stack: shears, wantTicks: 8},
		{name: "shears clear leaves in a tick", block: block.Leaves{}, stack: shears, wantTicks: 1},
		{name: "a sword is slower than shears on leaves", block: block.Leaves{}, stack: sword, wantTicks: 4},
		{name: "a bare hand is slower still", block: block.Leaves{}, stack: item.Stack{}, wantTicks: 6},
		{name: "a sword cuts cobweb like shears", block: block.Cobweb{}, stack: sword, wantTicks: 8},
		{name: "a sword fells bamboo in a tick", block: block.Bamboo{}, stack: sword, wantTicks: 1},
		{name: "a sword carves a pumpkin in a second", block: block.Pumpkin{}, stack: sword, wantTicks: 20},
		{name: "a sword mines stone no faster than a bare hand", block: block.Stone{}, stack: sword, wantTicks: 150},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := block.BreakDuration(tt.block, tt.stack, block.BreakContext{}).Milliseconds() / 50; got != tt.wantTicks {
				t.Errorf("got %d ticks, want %d", got, tt.wantTicks)
			}
		})
	}
}
