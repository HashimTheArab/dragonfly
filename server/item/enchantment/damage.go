package enchantment

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"math"
	"math/rand"
)

// AffectedDamageSource represents a world.DamageSource whose damage may be
// affected by an enchantment. A world.DamageSource does not need to implement
// AffectedDamageSource to let protection affect the damage. This happens
// depending on the (world.DamageSource).ReducedByResistance() method.
type AffectedDamageSource interface {
	world.DamageSource
	// AffectedByEnchantment specifies if a world.DamageSource is affected by
	// the item.EnchantmentType passed.
	AffectedByEnchantment(e item.EnchantmentType) bool
}

// DamageModifier is an item.EnchantmentType that can reduce damage through a
// modifier if an AffectedDamageSource returns true for it.
type DamageModifier interface {
	Modifier() float64
}

// ProtectionFactor calculates the combined protection factor for a slice of
// item.Enchantment. The factor depends on the world.DamageSource passed and is
// in a range of [0, 0.8], where 0.8 means incoming damage would be reduced by
// 80%.
// Formula matches PocketMine-MP's EPF (Enchantment Protection Factor) system:
// - EPF per piece = floor((6 + level²) * typeModifier / 3)
// - Total EPF capped at 25, then averaged (0.75 for consistency), capped at 20
// - Each EPF point = 4% damage reduction
func ProtectionFactor(src world.DamageSource, enchantments []item.Enchantment) float64 {
	totalEPF := 0.0
	for _, e := range enchantments {
		t := e.Type()
		level := float64(e.Level())

		reduced := false
		typeModifier := 1.0

		if _, ok := t.(protection); ok && src.ReducedByResistance() {
			// General Protection: typeModifier = 1.0
			reduced = true
			typeModifier = 1.0
		} else if _, ok := t.(blastProtection); ok {
			if asrc, ok := src.(AffectedDamageSource); ok && asrc.AffectedByEnchantment(t) {
				reduced = true
				typeModifier = 2.0
			}
		} else if _, ok := t.(fireProtection); ok {
			if asrc, ok := src.(AffectedDamageSource); ok && asrc.AffectedByEnchantment(t) {
				reduced = true
				typeModifier = 2.0
			}
		} else if _, ok := t.(projectileProtection); ok {
			if asrc, ok := src.(AffectedDamageSource); ok && asrc.AffectedByEnchantment(t) {
				reduced = true
				typeModifier = 1.5
			}
		} else if _, ok := t.(featherFalling); ok {
			if asrc, ok := src.(AffectedDamageSource); ok && asrc.AffectedByEnchantment(t) {
				reduced = true
				typeModifier = 2.5
			}
		}

		if reduced {
			// PM formula: floor((6 + level²) * typeModifier / 3)
			epf := math.Floor((6 + level*level) * typeModifier / 3)
			totalEPF += epf
		}
	}

	// Cap total EPF at 25
	if totalEPF > 25 {
		totalEPF = 25
	}
	// PM uses random 50-100%: mt_rand(50, 100) / 100
	randomFactor := float64(rand.Intn(51)+50) / 100.0
	totalEPF *= randomFactor
	// Ceiling the result (PM uses ceil())
	totalEPF = math.Ceil(totalEPF)
	// Cap effective EPF at 20
	if totalEPF > 20 {
		totalEPF = 20
	}
	// Each EPF point = 4% reduction
	return totalEPF * 0.04
}
