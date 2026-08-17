package item

import (
	"time"

	"github.com/df-mc/dragonfly/server/world"
)

// Sword is a tool generally used to attack enemies. In addition, it may be used to mine any block slightly
// faster than without tool and to break cobwebs rapidly.
type Sword struct {
	// Tier is the tier of the sword.
	Tier ToolTier
}

// AttackDamage returns the attack damage to the sword.
func (s Sword) AttackDamage() float64 {
	return s.Tier.BaseAttackDamage + 3
}

// MaxCount always returns 1.
func (s Sword) MaxCount() int {
	return 1
}

// ToolType returns the tool type for swords.
func (s Sword) ToolType() ToolType {
	return TypeSword
}

// HarvestLevel returns the harvest level of the sword tier.
func (s Sword) HarvestLevel() int {
	return s.Tier.HarvestLevel
}

// EnchantmentValue ...
func (s Sword) EnchantmentValue() int {
	return s.Tier.EnchantmentValue
}

// SwordMineable is implemented by blocks a sword cuts faster than a bare hand. Unlike a pickaxe or an
// axe, a sword has no single speed: the block it is cutting decides, from cobweb down to the plants it
// merely brushes aside.
type SwordMineable interface {
	// SwordMiningSpeed returns the speed a sword mines this block at.
	SwordMiningSpeed() float64
}

// BaseMiningEfficiency returns the speed the block passed names for a sword, or 1 for a block that names
// none.
func (s Sword) BaseMiningEfficiency(b world.Block) float64 {
	if m, ok := b.(SwordMineable); ok {
		return m.SwordMiningSpeed()
	}
	return 1
}

// DurabilityInfo ...
func (s Sword) DurabilityInfo() DurabilityInfo {
	return DurabilityInfo{
		MaxDurability:    s.Tier.Durability,
		BrokenItem:       simpleItem(Stack{}),
		AttackDurability: 1,
		BreakDurability:  2,
	}
}

// SmeltInfo ...
func (s Sword) SmeltInfo() SmeltInfo {
	switch s.Tier {
	case ToolTierIron:
		return newOreSmeltInfo(NewStack(IronNugget{}, 1), 0.1)
	case ToolTierGold:
		return newOreSmeltInfo(NewStack(GoldNugget{}, 1), 0.1)
	case ToolTierCopper:
		return newOreSmeltInfo(NewStack(CopperNugget{}, 1), 0.1)
	}
	return SmeltInfo{}
}

// FuelInfo ...
func (s Sword) FuelInfo() FuelInfo {
	if s.Tier == ToolTierWood {
		return newFuelInfo(time.Second * 10)
	}
	return FuelInfo{}
}

// RepairableBy ...
func (s Sword) RepairableBy(i Stack) bool {
	return toolTierRepairable(s.Tier)(i)
}

// EncodeItem ...
func (s Sword) EncodeItem() (name string, meta int16) {
	return "minecraft:" + s.Tier.Name + "_sword", 0
}
