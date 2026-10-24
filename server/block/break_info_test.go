package block_test

import (
	"math"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/enchantment"
	"github.com/df-mc/dragonfly/server/world"
)

// unbreakable is a block with the negative hardness Bedrock gives blocks that accumulate no destroy
// progress at all, as opposed to the zero hardness that breaks instantly.
type unbreakable struct{ block.Stone }

func (unbreakable) BreakInfo() block.BreakInfo {
	return block.BreakInfo{
		Hardness:    -1,
		Harvestable: func(item.Tool) bool { return true },
		Effective:   func(item.Tool) bool { return true },
		Drops:       func(item.Tool, []item.Enchantment) []item.Stack { return nil },
	}
}

// TestBreakDuration verifies the Bedrock Edition breaking calculation against the reference values
// documented at https://minecraft.wiki/w/Breaking#Calculation.
func TestBreakDuration(t *testing.T) {
	diamondPick := item.NewStack(item.Pickaxe{Tier: item.ToolTierDiamond}, 1)
	efficiencyDiamondPick := diamondPick.WithEnchantments(item.NewEnchantment(enchantment.Efficiency, 3))
	woodPick := item.NewStack(item.Pickaxe{Tier: item.ToolTierWood}, 1)
	efficiencyWoodPick := woodPick.WithEnchantments(item.NewEnchantment(enchantment.Efficiency, 3))

	tests := []struct {
		name      string
		block     world.Block
		stack     item.Stack
		ctx       block.BreakContext
		wantTicks int64
	}{
		{
			name:      "efficiency adds to best harvestable tool speed",
			block:     block.Stone{},
			stack:     efficiencyDiamondPick,
			wantTicks: 3,
		},
		{
			name:      "haste applies speed and damage multipliers",
			block:     block.Stone{},
			stack:     diamondPick,
			ctx:       block.BreakContext{HasteLevel: 1},
			wantTicks: 4,
		},
		{
			name:      "grounded best tool",
			block:     block.Stone{},
			stack:     diamondPick,
			wantTicks: 6,
		},
		{
			name:      "aqua affinity removes water penalty",
			block:     block.Stone{},
			stack:     diamondPick,
			ctx:       block.BreakContext{Underwater: true, AquaAffinity: true},
			wantTicks: 6,
		},
		{
			name:      "mining fatigue applies speed and damage multipliers",
			block:     block.Stone{},
			stack:     diamondPick,
			ctx:       block.BreakContext{MiningFatigueLevel: 1},
			wantTicks: 27,
		},
		{
			name:      "airborne penalty before rounding",
			block:     block.Stone{},
			stack:     diamondPick,
			ctx:       block.BreakContext{Airborne: true},
			wantTicks: 29,
		},
		{
			name:      "a tool fast enough to break in one tick still costs that tick",
			block:     block.Netherrack{},
			stack:     efficiencyDiamondPick,
			wantTicks: 1,
		},
		{
			name:      "airborne slows the same soft block below one tick",
			block:     block.Netherrack{},
			stack:     efficiencyDiamondPick,
			ctx:       block.BreakContext{Airborne: true},
			wantTicks: 4,
		},
		{
			name:      "water without aqua affinity slows mining",
			block:     block.Stone{},
			stack:     diamondPick,
			ctx:       block.BreakContext{Underwater: true},
			wantTicks: 29,
		},
		{
			name:      "riding slows mining the way being airborne does",
			block:     block.Stone{},
			stack:     diamondPick,
			ctx:       block.BreakContext{Riding: true},
			wantTicks: 29,
		},
		{
			name:      "flight suppresses only the airborne penalty, not the riding one",
			block:     block.Stone{},
			stack:     diamondPick,
			ctx:       block.BreakContext{Riding: true, Airborne: true, Flying: true},
			wantTicks: 29,
		},
		{
			// A wooden pickaxe mines diamond ore at its own speed of 2, not at the bare hand's 1, even
			// though its tier is too low to harvest it.
			name:      "a tool of too low a tier still mines at its own speed",
			block:     block.DiamondOre{Type: block.StoneOre()},
			stack:     woodPick,
			wantTicks: 150,
		},
		{
			// Efficiency counts at a tier too low to harvest as well, here giving a progress of 0.04 that
			// the client's running total passes in 25 additions where 1/0.04 rounds up to 26. The tick
			// count has to come from that total, not from a division.
			name:      "efficiency applies at a tier too low to harvest",
			block:     block.DiamondOre{Type: block.StoneOre()},
			stack:     efficiencyWoodPick,
			wantTicks: 25,
		},
		{
			name:      "zero hardness breaks instantly",
			block:     block.ShortGrass{},
			stack:     item.Stack{},
			wantTicks: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := block.BreakDuration(tt.block, tt.stack, tt.ctx).Milliseconds() / 50; got != tt.wantTicks {
				t.Errorf("got %d ticks, want %d", got, tt.wantTicks)
			}
		})
	}
}

// TestBreakDurationUnbreakable verifies that a negative hardness never breaks, rather than breaking
// instantly the way zero hardness does.
func TestBreakDurationUnbreakable(t *testing.T) {
	pick := item.NewStack(item.Pickaxe{Tier: item.ToolTierNetherite}, 1)
	for _, b := range []world.Block{unbreakable{}, block.Bedrock{}} {
		if got := block.BreakDuration(b, pick, block.BreakContext{}); got != math.MaxInt64 {
			t.Errorf("%T: got %v, want math.MaxInt64", b, got)
		}
	}
	if block.BreaksInstantly(unbreakable{}) {
		t.Error("unbreakable block reported as breaking instantly")
	}
}

// TestBreakDurationMiningFatigueStalls verifies that mining fatigue strong enough to round the destroy
// progress off the running total leaves the block unbreakable rather than overflowing the duration.
func TestBreakDurationMiningFatigueStalls(t *testing.T) {
	pick := item.NewStack(item.Pickaxe{Tier: item.ToolTierNetherite}, 1)
	ctx := block.BreakContext{MiningFatigueLevel: 20}
	if got := block.BreakDuration(block.Stone{}, pick, ctx); got != math.MaxInt64 {
		t.Errorf("got %v, want math.MaxInt64", got)
	}
}

// TestBreaksInstantly verifies that BreaksInstantly reports an instant break only for zero-hardness blocks,
// not for positive-hardness blocks that merely break within one tick due to a fast tool.
func TestBreaksInstantly(t *testing.T) {
	tests := []struct {
		name  string
		block world.Block
		want  bool
	}{
		{
			name:  "zero-hardness block breaks instantly",
			block: block.ShortGrass{},
			want:  true,
		},
		{
			name:  "positive-hardness block does not break instantly",
			block: block.Netherrack{},
			want:  false,
		},
		{
			name:  "stone does not break instantly",
			block: block.Stone{},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := block.BreaksInstantly(tt.block); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
