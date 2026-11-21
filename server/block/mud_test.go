package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// Mud lowers a standing body without changing the height of packed mud.
func TestMud_CollisionHeight(t *testing.T) {
	for _, tc := range []struct {
		name   string
		block  world.Block
		height float64
	}{
		{"mud", Mud{}, .875},
		{"packed_mud", PackedMud{}, 1},
		{"dirt", Dirt{}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			boxes := tc.block.Model().BBox(cube.Pos{}, nil)
			if len(boxes) != 1 || boxes[0] != cube.Box(0, 0, 0, 1, tc.height, 1) {
				t.Fatalf("collision boxes = %v, want height %v", boxes, tc.height)
			}
		})
	}
}
