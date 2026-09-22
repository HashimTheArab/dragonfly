package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// Replantable is implemented by plants that are harvested by breaking them once
// fully grown and are replanted from their own item against the block that held
// them. Melon and pumpkin stems are not replantable: their harvest is the fruit
// they grow, not the stem.
type Replantable interface {
	world.Block
	// FullyGrown reports whether breaking the plant now yields its full harvest.
	FullyGrown() bool
	// Support returns the position of the block holding a plant at pos, and the
	// face of that block the plant is attached to.
	Support(pos cube.Pos) (cube.Pos, cube.Face)
	// SupportedBy reports whether b can hold the plant.
	SupportedBy(b world.Block) bool
}

// Support returns the farmland beneath a crop at pos.
func (crop) Support(pos cube.Pos) (cube.Pos, cube.Face) {
	return pos.Side(cube.FaceDown), cube.FaceUp
}

// SupportedBy reports whether b is farmland, the only block a crop grows on.
func (crop) SupportedBy(b world.Block) bool {
	_, ok := b.(Farmland)
	return ok
}

// fullyGrown reports whether a crop has reached its final growth stage.
func (c crop) fullyGrown() bool {
	return c.Growth == 7
}

// FullyGrown reports whether the wheat is ready to harvest.
func (s WheatSeeds) FullyGrown() bool { return s.fullyGrown() }

// FullyGrown reports whether the carrots are ready to harvest.
func (c Carrot) FullyGrown() bool { return c.fullyGrown() }

// FullyGrown reports whether the potatoes are ready to harvest.
func (p Potato) FullyGrown() bool { return p.fullyGrown() }

// FullyGrown reports whether the beetroot is ready to harvest.
func (b BeetrootSeeds) FullyGrown() bool { return b.fullyGrown() }

// FullyGrown reports whether the nether wart is ready to harvest.
func (n NetherWart) FullyGrown() bool { return n.Age == 3 }

// Support returns the soul sand beneath nether wart at pos.
func (NetherWart) Support(pos cube.Pos) (cube.Pos, cube.Face) {
	return pos.Side(cube.FaceDown), cube.FaceUp
}

// SupportedBy reports whether b is soul sand, the only block nether wart grows on.
func (NetherWart) SupportedBy(b world.Block) bool {
	_, ok := b.(SoulSand)
	return ok
}

// FullyGrown reports whether the cocoa pod is ready to harvest.
func (c CocoaBean) FullyGrown() bool { return c.Age == 2 }

// Support returns the log a cocoa pod at pos hangs from, and the face of that
// log the pod is attached to.
func (c CocoaBean) Support(pos cube.Pos) (cube.Pos, cube.Face) {
	return pos.Side(c.Facing.Face()), c.Facing.Opposite().Face()
}

// SupportedBy reports whether b is a jungle log or jungle wood, the only blocks
// a cocoa pod grows on.
func (CocoaBean) SupportedBy(b world.Block) bool {
	switch wood := b.(type) {
	case Log:
		return wood.Wood == JungleWood()
	case Wood:
		return wood.Wood == JungleWood()
	}
	return false
}
