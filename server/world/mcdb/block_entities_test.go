package mcdb

import (
	"io"
	"log/slog"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/goleveldb/leveldb"
)

func TestStoreBlockEntitiesHandlesNilData(t *testing.T) {
	db := &DB{conf: Config{Log: slog.New(slog.NewTextHandler(io.Discard, nil))}}
	batch := leveldb.MakeBatch(1)

	db.storeBlockEntities(batch, dbKey{pos: world.ChunkPos{1, 2}, dim: world.Overworld}, []chunk.BlockEntity{
		{Pos: cube.Pos{3, 4, 5}},
	})

	if batch.Len() != 1 {
		t.Fatalf("expected one batch entry, got %d", batch.Len())
	}
}
