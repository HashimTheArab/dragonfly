package block

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// BlocksUnder returns the corners of the range of block positions directly below an entity with the bounding box
// passed. Every block in that range is one the entity stands on: an entity narrower than a block may rest on the edge
// of one with its centre over the block beside it, and looking only below its centre would miss the block it is
// actually standing on.
func BlocksUnder(box cube.BBox) (low, high cube.Pos) {
	// The Y is taken from the box itself, while the horizontal range is taken from a slightly smaller box so that an
	// entity resting exactly on the boundary between two blocks does not reach into the column beside the one it
	// stands on.
	y := int(math.Floor(box.Min()[1] - 0.0001))
	horizontal := box.Grow(-0.0001)
	low, high = cube.PosFromVec3(horizontal.Min()), cube.PosFromVec3(horizontal.Max())
	low[1], high[1] = y, y
	return low, high
}

// LandedOn returns the first EntityLander among the blocks directly below an entity with the bounding box passed, the
// block whose EntityLand a fall ending in that box runs, along with its position.
func LandedOn(src world.BlockSource, box cube.BBox) (cube.Pos, EntityLander, bool) {
	low, high := BlocksUnder(box)
	for x := low[0]; x <= high[0]; x++ {
		for z := low[2]; z <= high[2]; z++ {
			pos := cube.Pos{x, low[1], z}
			if lander, ok := src.Block(pos).(EntityLander); ok {
				return pos, lander, true
			}
		}
	}
	return cube.Pos{}, nil, false
}
