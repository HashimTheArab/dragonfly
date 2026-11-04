package block

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

// cropDrop defines one possible crop item and its random count. A nil count
// produces one item; a count of zero omits the item from that harvest.
type cropDrop struct {
	item  world.Item
	count func([]item.Enchantment) int
}

// cropBreakInfo derives random drops and deterministic possible identities from
// the same definitions. Only items possible in this growth state are supplied.
func cropBreakInfo(hardness float64, effective func(item.Tool) bool, drops ...cropDrop) BreakInfo {
	info := newBreakInfo(hardness, alwaysHarvestable, effective, func(_ item.Tool, enchantments []item.Enchantment) []item.Stack {
		stacks := make([]item.Stack, 0, len(drops))
		for _, drop := range drops {
			count := 1
			if drop.count != nil {
				count = drop.count(enchantments)
			}
			if count > 0 {
				stacks = append(stacks, item.NewStack(drop.item, count))
			}
		}
		return stacks
	})
	info.PossibleDrops = make([]world.Item, len(drops))
	for i, drop := range drops {
		info.PossibleDrops[i] = drop.item
	}
	return info
}

// cropSeedDrops defines wheat/beetroot produce and seeds. Mature seed counts use
// binomial distribution B(3+fortune, 8/15), so seeds may be absent from a harvest.
func cropSeedDrops(seed, crop world.Item, growth int) []cropDrop {
	if growth < 7 {
		return []cropDrop{{item: seed}}
	}
	return []cropDrop{
		{item: crop},
		{item: seed, count: func(enchantments []item.Enchantment) int {
			return fortuneBinomial(3 + fortuneLevel(enchantments))
		}},
	}
}
