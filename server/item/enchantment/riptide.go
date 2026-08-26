package enchantment

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

// Riptide is a trident enchantment that launches the wielder while the trident
// is used in water or rain.
var Riptide riptide

type riptide struct{}

// Name ...
func (riptide) Name() string {
	return "Riptide"
}

// MaxLevel ...
func (riptide) MaxLevel() int {
	return 3
}

// Cost ...
func (riptide) Cost(level int) (int, int) {
	minCost := 17 + (level-1)*7
	return minCost, minCost + 50
}

// Rarity ...
func (riptide) Rarity() item.EnchantmentRarity {
	return item.EnchantmentRarityRare
}

// CompatibleWithEnchantment ...
func (riptide) CompatibleWithEnchantment(t item.EnchantmentType) bool {
	// TODO: Loyalty and Channeling.
	return true
}

// CompatibleWithItem ...
func (riptide) CompatibleWithItem(i world.Item) bool {
	t, ok := i.(interface{ Trident() bool })
	return ok && t.Trident()
}
