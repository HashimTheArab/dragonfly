package item

// Shield is an item used to block incoming attacks.
// TODO: Blocking (raising the shield, the damage it absorbs and the axe disable).
type Shield struct{}

// MaxCount always returns 1.
func (Shield) MaxCount() int {
	return 1
}

// OffHand always returns true.
func (Shield) OffHand() bool {
	return true
}

// DurabilityInfo ...
func (Shield) DurabilityInfo() DurabilityInfo {
	return DurabilityInfo{
		MaxDurability: 336,
		BrokenItem:    simpleItem(Stack{}),
	}
}

// EncodeItem ...
func (Shield) EncodeItem() (name string, meta int16) {
	return "minecraft:shield", 0
}
