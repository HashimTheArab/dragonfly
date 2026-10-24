package world

import "github.com/df-mc/dragonfly/server/block/cube"

// PlacementActor is an actor's type identifier and world-space bounding box.
// Bounds must not be shrunk: placement applies its own intersection tolerance.
type PlacementActor struct {
	Type   string
	Bounds cube.BBox
}

// PlacementObstructed reports whether actor prevents b from being placed at pos.
// Dropped items, arrows and experience orbs do not obstruct placement. Other
// identifiers, including unknown custom actors, use their supplied bounds.
func PlacementObstructed(src BlockSource, pos cube.Pos, b Block, actor PlacementActor) bool {
	switch actor.Type {
	case "minecraft:item", "minecraft:arrow", "minecraft:xp_orb":
		return false
	}
	boxes := b.Model().BBox(pos, src)
	for i, box := range boxes {
		boxes[i] = box.Translate(pos.Vec3())
	}
	return cube.AnyIntersections(boxes, actor.Bounds.Grow(-1e-4))
}
