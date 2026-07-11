package chunk_test

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

func TestChunkClearBlockEntityDataInRange(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	ch := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	clearedPos := cube.Pos{32, 64, 48}
	keptPos := cube.Pos{32, 80, 48}
	outsideXZ := cube.Pos{48, 64, 48}

	ch.SetBlockEntityData(clearedPos, map[string]any{"id": "Chest"})
	ch.SetBlockEntityData(keptPos, map[string]any{"id": "Chest"})
	ch.SetBlockEntityData(outsideXZ, map[string]any{"id": "Chest"})

	ch.ClearBlockEntityDataInRange(cube.Pos{32, 64, 48}, cube.Pos{47, 79, 63})

	if _, ok := ch.BlockEntityData(clearedPos); ok {
		t.Fatal("expected block entity data inside range to be removed")
	}
	if _, ok := ch.BlockEntityData(keptPos); !ok {
		t.Fatal("expected block entity data outside Y range to remain")
	}
	if _, ok := ch.BlockEntityData(outsideXZ); !ok {
		t.Fatal("expected block entity data outside X/Z range to remain")
	}
}

func TestChunkClonePreservesIndependentBlockEntityData(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	pos := cube.Pos{1, 64, 1}
	ch := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	ch.SetBlockEntityData(pos, map[string]any{"id": "Chest"})

	clone := ch.Clone()
	if _, ok := clone.BlockEntityData(pos); !ok {
		t.Fatal("expected clone to preserve block entity data")
	}
	clone.SetBlockEntityData(pos, nil)
	if _, ok := ch.BlockEntityData(pos); !ok {
		t.Fatal("expected clone block entity mutations not to affect the original")
	}
}
