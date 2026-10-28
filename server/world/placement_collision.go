package world

import "github.com/df-mc/dragonfly/server/block/cube"

// PlacementActor is an actor's entity type and world-space bounding box. A nil
// Type is an actor without a registered EntityType, which always uses its bounds.
// Bounds must not be shrunk: placement applies its own intersection tolerance.
type PlacementActor struct {
	Type   EntityType
	Bounds cube.BBox
}

// PlacementPassableType is implemented by an EntityType whose entities never
// prevent a block from being placed inside their bounds, such as dropped items.
type PlacementPassableType interface {
	EntityType
	PassableByPlacement() bool
}

// PlacementModel is the world-space model of a block about to be placed, used
// to test which actors obstruct it.
type PlacementModel []cube.BBox

// NewPlacementModel returns the world-space model boxes of b placed at pos.
func NewPlacementModel(src BlockSource, pos cube.Pos, b Block) PlacementModel {
	boxes := b.Model().BBox(pos, src)
	for i, box := range boxes {
		boxes[i] = box.Translate(pos.Vec3())
	}
	return boxes
}

// Obstructed reports whether actor prevents the block from being placed.
// An actor whose Type is a passable PlacementPassableType never obstructs.
func (m PlacementModel) Obstructed(actor PlacementActor) bool {
	if passable, ok := actor.Type.(PlacementPassableType); ok && passable.PassableByPlacement() {
		return false
	}
	return cube.AnyIntersections(m, actor.Bounds.Grow(-1e-4))
}
