package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"testing"
)

func TestSoulSand_CollisionHeight(t *testing.T) {
	// BDS 1.26.45.1: a player on soul sand at y=-61 stands at -60.125,
	// rather than -60. Lens SoulSandBlock::getCollisionShape (26.30,
	// RVA 0xae64570) uses its own BLOCK_AABB rather than a full-cube shape.
	pos := cube.Pos{}
	boxes := (SoulSand{}).Model().BBox(pos, nil)
	if len(boxes) != 1 || boxes[0] != cube.Box(0, 0, 0, 1, 0.875, 1) {
		t.Fatalf("soul sand collision = %v", boxes)
	}
	if !(SoulSand{}).Model().FaceSolid(pos, cube.FaceUp, nil) {
		t.Fatal("soul sand must still support block attachments")
	}
	if got := (SoulSoil{}).Model().BBox(pos, nil); len(got) != 1 || got[0] != cube.Box(0, 0, 0, 1, 1, 1) {
		t.Fatalf("soul soil collision = %v", got)
	}
}
