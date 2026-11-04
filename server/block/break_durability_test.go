package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

func TestBreakDurability(t *testing.T) {
	pick := item.NewStack(item.Pickaxe{Tier: item.ToolTierDiamond}, 1)
	for _, tt := range []struct {
		name string
		b    world.Block
		held item.Stack
		want int
	}{
		{"budding amethyst", BuddingAmethyst{}, pick, 1},
		{"stone", Stone{}, pick, 1},
		{"instant", ShortGrass{}, pick, 0},
		{"empty hand", Stone{}, item.Stack{}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := BreakDurability(tt.b, tt.held); got != tt.want {
				t.Fatalf("got %d want %d", got, tt.want)
			}
		})
	}
}
