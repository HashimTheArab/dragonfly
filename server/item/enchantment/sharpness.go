package enchantment

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

// Sharpness is an enchantment applied to a sword or axe that increases melee
// damage.
var Sharpness sharpness

type sharpness struct{}

// Name ...
func (sharpness) Name() string {
	return "Sharpness"
}

// MaxLevel ...
func (sharpness) MaxLevel() int {
	return 5
}

// Cost ...
func (sharpness) Cost(level int) (int, int) {
	minCost := 1 + (level-1)*11
	return minCost, minCost + 20
}

// Rarity ...
func (sharpness) Rarity() item.EnchantmentRarity {
	return item.EnchantmentRarityCommon
}

// Addend returns the additional damage when attacking with sharpness.
// Formula matches PocketMine-MP: 0.5 * (level + 1)
func (sharpness) Addend(level int) float64 {
	return 0.5 * float64(level+1)
}

// CompatibleWithEnchantment ...
func (sharpness) CompatibleWithEnchantment(item.EnchantmentType) bool {
	return true
}

// CompatibleWithItem ...
func (sharpness) CompatibleWithItem(i world.Item) bool {
	t, ok := i.(item.Tool)
	return ok && (t.ToolType() == item.TypeSword || t.ToolType() == item.TypeAxe)
}
