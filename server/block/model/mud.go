package model

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// Mud has a seven-eighths-high collision box.
type Mud struct{ Solid }

// BBox returns the lowered surface on which entities stand.
func (Mud) BBox(cube.Pos, world.BlockSource) []cube.BBox {
	return []cube.BBox{full.ExtendTowards(cube.FaceUp, -.125)}
}
