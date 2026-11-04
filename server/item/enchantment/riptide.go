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
	id, registered := item.EnchantmentID(t)
	return !registered || (id != loyaltyID && id != channelingID)
}

// CompatibleWithItem ...
func (riptide) CompatibleWithItem(i world.Item) bool {
	t, ok := i.(item.TridentType)
	return ok && t.Trident()
}
