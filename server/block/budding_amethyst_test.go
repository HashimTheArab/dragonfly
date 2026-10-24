package block

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/enchantment"
	"github.com/df-mc/dragonfly/server/world"
	"testing"
	"time"
)

func TestBuddingAmethyst_Mining(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	b, ok := world.BlockByName("minecraft:budding_amethyst", nil)
	if !ok {
		t.Fatal("missing vanilla block")
	}
	breakable, ok := b.(Breakable)
	if !ok {
		t.Fatalf("budding amethyst is not breakable: %T", b)
	}
	pick := item.Pickaxe{Tier: item.ToolTierIron}
	if got := BreakDuration(b, item.NewStack(pick, 1), BreakContext{}); got != 8*50*time.Millisecond {
		t.Fatalf("iron pick duration = %v, want 8 ticks", got)
	}
	for _, enchants := range [][]item.Enchantment{nil, {item.NewEnchantment(enchantment.SilkTouch, 1)}} {
		if drops := breakable.BreakInfo().Drops(pick, enchants); len(drops) != 0 {
			t.Fatalf("budding amethyst dropped items: %v", drops)
		}
	}
}
