package enchantment

import (
	"github.com/df-mc/dragonfly/server/item"
	"testing"
)

func TestDamageItem(t *testing.T) {
	pick := item.NewStack(item.Pickaxe{Tier: item.ToolTierDiamond}, 1)
	for _, tt := range []struct {
		name   string
		stack  item.Stack
		damage int
		want   int
		empty  bool
	}{
		{"ordinary", pick, 1, pick.Durability() - 1, false},
		{"breaks", pick.WithDurability(1), 1, 0, true},
		{"unbreakable", pick.AsUnbreakable(), 1, pick.Durability(), false},
		{"no cost", pick, 0, pick.Durability(), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := DamageItem(tt.stack, tt.damage)
			if got.Empty() != tt.empty || (!tt.empty && got.Durability() != tt.want) {
				t.Fatalf("unexpected result: %v", got)
			}
		})
	}
}
