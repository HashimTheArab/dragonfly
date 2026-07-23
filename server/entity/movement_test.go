package entity

import (
	"context"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

func TestBlockBBoxsAroundExcludesPositiveOuterLayer(t *testing.T) {
	w := world.Config{Dim: world.End, Synchronous: true}.New()
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Errorf("close world: %v", err)
		}
	})

	err := w.Do(func(tx *world.Tx) {
		opts := &world.SetOpts{
			DisableBlockUpdates:       true,
			DisableLiquidDisplacement: true,
			DisableRedstoneUpdates:    true,
		}
		tx.SetBlock(cube.Pos{1, 0, 0}, block.Stone{}, opts)
		tx.SetBlock(cube.Pos{2, 0, 0}, block.Stone{}, opts)

		boxes := blockBBoxsAround(tx, cube.Box(0, 0, 0, 1, 1, 1))
		if len(boxes) != 1 {
			t.Fatalf("block BBoxes around unit box = %d, want 1", len(boxes))
		}
		if minX := boxes[0].Min()[0]; minX != 1 {
			t.Fatalf("returned block BBox starts at x=%v, want 1", minX)
		}
	}).Wait(context.Background())
	if err != nil {
		t.Fatalf("world task failed: %v", err)
	}
}
