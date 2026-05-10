package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// Azalea are a flower like block that can grow into an azalea tree.
type Azalea struct {
	solid
	transparent

	// Flowering specifies if this block is flowering.
	Flowering bool
}

// BoneMeal ...
func (a Azalea) BoneMeal(pos cube.Pos, tx *world.Tx) (success bool) {
	// TODO: Requires porting world.Feature and the vanilla azalea tree feature.
	return false
}

// NeighbourUpdateTick ...
func (a Azalea) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	if !supportsVegetation(a, tx.Block(pos.Side(cube.FaceDown))) {
		breakBlockNoDrops(a, pos, tx)
	}
}

// UseOnBlock ...
func (a Azalea) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, a)
	if !used {
		return false
	}
	down := tx.Block(pos.Side(cube.FaceDown))
	if !supportsVegetation(a, down) {
		return false
	}

	place(tx, pos, a, user, ctx)
	return placed(ctx)
}

// HasLiquidDrops ...
func (Azalea) HasLiquidDrops() bool {
	return true
}

// FlammabilityInfo ...
func (a Azalea) FlammabilityInfo() FlammabilityInfo {
	return newFlammabilityInfo(60, 100, false)
}

// BreakInfo ...
func (a Azalea) BreakInfo() BreakInfo {
	return newBreakInfo(0, alwaysHarvestable, nothingEffective, oneOf(a))
}

// CompostChance ...
func (Azalea) CompostChance() float64 {
	return 0.65
}

// EncodeItem ...
func (a Azalea) EncodeItem() (name string, meta int16) {
	name = "minecraft:"
	if a.Flowering {
		name += "flowering_"
	}
	name += "azalea"
	return name, 0
}

// EncodeBlock ...
func (a Azalea) EncodeBlock() (name string, properties map[string]any) {
	name = "minecraft:"
	if a.Flowering {
		name += "flowering_"
	}
	name += "azalea"
	return name, map[string]any{}
}

// allAzalea returns a list of all possible azalea states.
func allAzalea() (azaleas []world.Block) {
	f := func(flowering bool) {
		azaleas = append(azaleas, Azalea{Flowering: flowering})
	}
	f(true)
	f(false)
	return azaleas
}
