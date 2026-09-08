package block

// BuddingAmethyst is the solid amethyst block on which amethyst buds grow.
// Bud growth is not yet implemented.
type BuddingAmethyst struct {
	solid
}

// BreakInfo permits mining with a pickaxe but never drops the block, including with Silk Touch.
func (BuddingAmethyst) BreakInfo() BreakInfo {
	return newBreakInfo(1.5, pickaxeHarvestable, pickaxeHarvestable, simpleDrops())
}

// EncodeItem ...
func (BuddingAmethyst) EncodeItem() (string, int16) {
	return "minecraft:budding_amethyst", 0
}

// EncodeBlock ...
func (BuddingAmethyst) EncodeBlock() (string, map[string]any) {
	return "minecraft:budding_amethyst", nil
}
