package block

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

// BreakDurability returns the durability cost of breaking b with held, before
// creative-mode immunity, Unbreaking, or item-damage event handlers apply.
// Naturally instant blocks cost nothing, even when the held item is durable.
func BreakDurability(b world.Block, held item.Stack) int {
	if BreaksInstantly(b) {
		return 0
	}
	if durable, ok := held.Item().(item.Durable); ok {
		return durable.DurabilityInfo().BreakDurability
	}
	return 0
}
