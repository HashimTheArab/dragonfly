package model

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// SoulSand has a seven-eighths-high entity collision box and solid faces.
type SoulSand struct{ Solid }

// BBox returns vanilla's lowered soul-sand collision shape.
func (SoulSand) BBox(cube.Pos, world.BlockSource) []cube.BBox {
	return []cube.BBox{full.ExtendTowards(cube.FaceUp, -0.125)}
}
