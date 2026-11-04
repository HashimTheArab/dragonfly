package item

// FishingRod is an item used to catch fish and to hook entities and blocks.
// TODO: Casting (the fishing hook entity, catching loot and reeling entities in).
type FishingRod struct{}

// MaxCount always returns 1.
func (FishingRod) MaxCount() int {
	return 1
}

// EnchantmentValue ...
func (FishingRod) EnchantmentValue() int {
	return 1
}

// DurabilityInfo ...
func (FishingRod) DurabilityInfo() DurabilityInfo {
	return DurabilityInfo{
		MaxDurability: 384,
		BrokenItem:    simpleItem(Stack{}),
	}
}

// EncodeItem ...
func (FishingRod) EncodeItem() (name string, meta int16) {
	return "minecraft:fishing_rod", 0
}
