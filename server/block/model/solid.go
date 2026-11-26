package model

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// Solid is the model of a fully solid block. Blocks with this model, such as stone or wooden planks, have a
// 1x1x1 collision box.
type Solid struct{}

// full is a BBox that occupies a full block.
var full = cube.Box(0, 0, 0, 1, 1, 1)

// BBox returns a physics.BBox spanning a full block.
func (Solid) BBox(cube.Pos, world.BlockSource) []cube.BBox {
	return []cube.BBox{full}
}

// FaceSolid always returns true.
func (Solid) FaceSolid(cube.Pos, cube.Face, world.BlockSource) bool {
	return true
}

func (Solid) solidBlock() {}

// IsSolid reports whether m is a solid block. This includes models embedding Solid whose entity collision box is
// lower than a full cube, such as soul sand and mud, which still support attachments, block spawning and cause
// suffocation.
func IsSolid(m world.BlockModel) bool {
	_, ok := m.(interface{ solidBlock() })
	return ok
}
