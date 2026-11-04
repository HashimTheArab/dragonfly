package item

import (
	"iter"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// EndCrystal is an item that can be placed on obsidian or bedrock to spawn an End crystal entity.
type EndCrystal struct{}

// endCrystalSupport represents a block that supports an End crystal.
type endCrystalSupport interface {
	SupportsEndCrystal() bool
}

// UseOnBlock places an End crystal on top of the clicked block if it is
// obsidian or bedrock, the two blocks above it are air and no other entities
// intersect the space the crystal is placed in. The face clicked is ignored.
func (e EndCrystal) UseOnBlock(pos cube.Pos, _ cube.Face, _ mgl64.Vec3, tx *world.Tx, _ User, ctx *UseContext) bool {
	boxes := func(yield func(cube.BBox) bool) {
		for entity := range tx.EntitiesWithin(e.placementBox(pos).Grow(2)) {
			if !yield(entity.H().Type().BBox(entity).Translate(entity.Position())) {
				return
			}
		}
	}
	if !e.CanPlace(pos, tx, tx.Range(), boxes) {
		return false
	}
	opts := world.EntitySpawnOpts{Position: e.PlacementPosition(pos)}
	tx.AddEntity(tx.World().EntityRegistry().Config().EndCrystal(opts))
	ctx.SubtractFromCount(1)
	return true
}

// CanPlace reports whether a crystal can be placed on pos, using the supplied
// terrain and world-space entity boxes. It applies the same support, air, bounds,
// and entity collision rules as UseOnBlock. Callers may supply preview terrain.
func (e EndCrystal) CanPlace(pos cube.Pos, blocks world.BlockSource, bounds cube.Range, entities iter.Seq[cube.BBox]) bool {
	above, twoAbove := pos.Side(cube.FaceUp), pos.Side(cube.FaceUp).Side(cube.FaceUp)
	if pos.OutOfBounds(bounds) || above.OutOfBounds(bounds) || twoAbove.OutOfBounds(bounds) {
		return false
	}
	support, ok := blocks.Block(pos).(endCrystalSupport)
	if !ok || !support.SupportsEndCrystal() {
		return false
	}
	if blocks.Block(above) != air() || blocks.Block(twoAbove) != air() {
		return false
	}
	box := e.placementBox(pos)
	for other := range entities {
		if box.IntersectsWith(other) {
			return false
		}
	}
	return true
}

// PlacementPosition returns the entity spawn point above a supporting block.
func (EndCrystal) PlacementPosition(pos cube.Pos) mgl64.Vec3 {
	return pos.Side(cube.FaceUp).Vec3Middle()
}

// placementBox returns the admission volume anchored at the clicked block.
func (EndCrystal) placementBox(pos cube.Pos) cube.BBox {
	return cube.Box(0, 0, 0, 1, 2, 1).Translate(pos.Vec3())
}

// EncodeItem ...
func (EndCrystal) EncodeItem() (name string, meta int16) {
	return "minecraft:end_crystal", 0
}
