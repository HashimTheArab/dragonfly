package item

// Mace is a weapon that deals extra damage based on the distance a player falls before hitting with it.
// TODO: Smash attacks (the fall-distance damage bonus, knockback and the fall damage it cancels).
type Mace struct{}

// AttackDamage ...
func (Mace) AttackDamage() float64 {
	return 5
}

// MaxCount always returns 1.
func (Mace) MaxCount() int {
	return 1
}

// EnchantmentValue ...
func (Mace) EnchantmentValue() int {
	return 15
}

// DurabilityInfo ...
func (Mace) DurabilityInfo() DurabilityInfo {
	return DurabilityInfo{
		MaxDurability:    500,
		BrokenItem:       simpleItem(Stack{}),
		AttackDurability: 1,
		BreakDurability:  2,
	}
}

// EncodeItem ...
func (Mace) EncodeItem() (name string, meta int16) {
	return "minecraft:mace", 0
}
