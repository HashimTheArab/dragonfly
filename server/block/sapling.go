package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// Sapling is a non-solid block that grows into a tree when it has enough light and space.
// TODO: Growth (random ticking, bone meal and the tree shapes each wood type grows into).
type Sapling struct {
	empty
	transparent

	// Wood is the type of wood of the sapling.
	Wood WoodType
	// AgeBit is toggled every time the sapling passes a growth check. The tree only appears on a check that passes
	// while the bit is already set.
	AgeBit bool
}

// NeighbourUpdateTick ...
func (s Sapling) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	if !supportsVegetation(s, tx.Block(pos.Side(cube.FaceDown))) {
		breakBlock(s, pos, tx)
	}
}

// UseOnBlock ...
func (s Sapling) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, s)
	if !used || !supportsVegetation(s, tx.Block(pos.Side(cube.FaceDown))) {
		return false
	}

	s.AgeBit = false
	place(tx, pos, s, user, ctx)
	return placed(ctx)
}

// HasLiquidDrops ...
func (Sapling) HasLiquidDrops() bool {
	return true
}

// FlammabilityInfo ...
func (s Sapling) FlammabilityInfo() FlammabilityInfo {
	return newFlammabilityInfo(30, 60, true)
}

// BreakInfo ...
func (s Sapling) BreakInfo() BreakInfo {
	return newBreakInfo(0, alwaysHarvestable, nothingEffective, oneOf(Sapling{Wood: s.Wood}))
}

// EncodeItem ...
func (s Sapling) EncodeItem() (name string, meta int16) {
	return "minecraft:" + s.Wood.String() + "_sapling", 0
}

// EncodeBlock ...
func (s Sapling) EncodeBlock() (string, map[string]any) {
	return "minecraft:" + s.Wood.String() + "_sapling", map[string]any{"age_bit": s.AgeBit}
}

// allSaplings returns a list of all sapling variants.
func allSaplings() (saplings []world.Block) {
	for _, w := range WoodTypes() {
		if !w.Sapling() {
			continue
		}
		saplings = append(saplings, Sapling{Wood: w})
		saplings = append(saplings, Sapling{Wood: w, AgeBit: true})
	}
	return
}
