package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

// RootedDirt is a natural decorative block that can generate under azalea trees.
type RootedDirt struct {
	solid
}

// SoilFor ...
func (r RootedDirt) SoilFor(block world.Block) bool {
	switch block.(type) {
	case ShortGrass, Fern, DoubleTallGrass, DeadBush, Flower, DoubleFlower, NetherSprouts, PinkPetals, SugarCane, Azalea, Sapling:
		return true
	}
	return false
}

// BreakInfo ...
func (r RootedDirt) BreakInfo() BreakInfo {
	return newBreakInfo(0.5, alwaysHarvestable, shovelEffective, oneOf(r))
}

// Till ...
func (r RootedDirt) Till() (world.Block, bool) {
	return Dirt{}, true
}

// AfterTill drops hanging roots after rooted dirt is tilled.
func (r RootedDirt) AfterTill(pos cube.Pos, tx *world.Tx) {
	dropItem(tx, item.NewStack(HangingRoots{}, 1), pos.Vec3Centre())
}

// BoneMeal ...
func (r RootedDirt) BoneMeal(pos cube.Pos, tx *world.Tx) (success bool) {
	pos2 := pos.Side(cube.FaceDown)
	if _, ok := tx.Block(pos2).(Air); !ok {
		return false
	}
	tx.SetBlock(pos2, HangingRoots{}, nil)
	return true
}

// EncodeItem ...
func (r RootedDirt) EncodeItem() (name string, meta int16) {
	return "minecraft:dirt_with_roots", 0
}

// EncodeBlock ...
func (r RootedDirt) EncodeBlock() (string, map[string]any) {
	return "minecraft:dirt_with_roots", nil
}
