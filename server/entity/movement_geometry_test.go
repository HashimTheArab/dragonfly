package entity

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

type geometryBlockSource map[cube.Pos]world.Block

// Block supplies explicit geometry and air for every other queried cell.
func (s geometryBlockSource) Block(pos cube.Pos) world.Block {
	if b, ok := s[pos]; ok {
		return b
	}
	return block.Air{}
}

func TestBlockBBoxesAround_PreservesFarPartialGeometry(t *testing.T) {
	const x = 8388609
	src := geometryBlockSource{cube.Pos{x, 63, 0}: block.Lantern{}}
	boxes := BlockBBoxesAround(src, cube.Box(x+.4, 63.45, .4, x+.6, 63.55, .6))
	want := cube.Box(x+.3125, 63, .3125, x+.6875, 63.5, .6875)
	if len(boxes) != 1 || boxes[0] != want {
		t.Fatalf("partial collision boxes = %v, want %v", boxes, want)
	}
}
