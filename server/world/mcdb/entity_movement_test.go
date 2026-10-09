package mcdb

import (
	"errors"
	"testing"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/goleveldb/leveldb"
)

func TestMovedActorSurvivesSourceColumnSave(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	source, destination := world.ChunkPos{}, world.ChunkPos{1, 0}
	empty := func() *chunk.Column {
		return &chunk.Column{Chunk: chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())}
	}
	actor := chunk.Entity{ID: 42, Data: map[string]any{"identifier": "minecraft:cow"}}
	original := empty()
	original.Entities = []chunk.Entity{actor}
	if err := db.StoreColumn(source, world.Overworld, original); err != nil {
		t.Fatal(err)
	}
	moved := empty()
	moved.Entities = []chunk.Entity{actor}
	// A destination may be saved before its dirty source column.
	if err := db.StoreColumn(destination, world.Overworld, moved); err != nil {
		t.Fatal(err)
	}
	if err := db.StoreColumn(source, world.Overworld, empty()); err != nil {
		t.Fatal(err)
	}
	got, err := db.entities(dbKey{pos: destination, dim: world.Overworld})
	if err != nil {
		t.Fatalf("destination actor was deleted by source save: %v", err)
	}
	if len(got) != 1 || got[0].ID != actor.ID {
		t.Fatalf("destination actors = %v", got)
	}
	if err := db.StoreColumn(destination, world.Overworld, empty()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ldb.Get(entityIndex(actor.ID), nil); err == nil {
		t.Fatal("removed actor data survived final owner removal")
	}
}

func TestLegacyActorRemovalAndMovement(t *testing.T) {
	for _, moved := range []bool{false, true} {
		t.Run(map[bool]string{false: "removed", true: "moved"}[moved], func(t *testing.T) {
			db, err := Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			source, destination := world.ChunkPos{}, world.ChunkPos{1, 0}
			actor := chunk.Entity{ID: 42, Data: map[string]any{"identifier": "minecraft:cow"}}
			col := &chunk.Column{Chunk: chunk.New(world.DefaultBlockRegistry, world.Overworld.Range()), Entities: []chunk.Entity{actor}}
			if err := db.StoreColumn(source, world.Overworld, col); err != nil {
				t.Fatal(err)
			}
			// Older saves have an actor index and payload but no Dragonfly owner key.
			if err := db.ldb.Delete(entityOwnerIndex(actor.ID), nil); err != nil {
				t.Fatal(err)
			}
			loaded, err := db.LoadColumn(source, world.Overworld)
			if err != nil || len(loaded.Entities) != 1 {
				t.Fatalf("load legacy actor: column=%v err=%v", loaded, err)
			}
			if moved {
				if err := db.StoreColumn(destination, world.Overworld, loaded); err != nil {
					t.Fatal(err)
				}
			}
			loaded.Entities = nil
			if err := db.StoreColumn(source, world.Overworld, loaded); err != nil {
				t.Fatal(err)
			}
			_, err = db.ldb.Get(entityIndex(actor.ID), nil)
			if moved && err != nil {
				t.Fatalf("moved legacy actor lost: %v", err)
			}
			if !moved && !errors.Is(err, leveldb.ErrNotFound) {
				t.Fatalf("removed legacy actor payload still exists: %v", err)
			}
		})
	}
}
