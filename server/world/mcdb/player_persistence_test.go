package mcdb_test

import (
	"encoding/binary"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/mcdb"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
)

func TestPlayerSnapshotDoesNotSurviveRejoin(t *testing.T) {
	dir := t.TempDir()
	id, pos := uuid.New(), mgl64.Vec3{1, 10, 1}
	// open reloads the saved world without starting background ticks or saves.
	open := func() (*world.World, *mcdb.DB) {
		db, err := mcdb.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		w := world.Config{Provider: db, Entities: entity.DefaultRegistry, Synchronous: true}.New()
		t.Cleanup(func() { w.Close() })
		return w, db
	}
	// join creates a new online player with the same persistent identity.
	join := func(tx *world.Tx) {
		tx.AddEntity(world.EntitySpawnOpts{ID: id}.New(player.Type, player.Config{UUID: id, Position: pos}))
	}
	w, _ := open()
	w.Do(join)
	w.Save()
	// Keep the player online through the final save, as in an autosave before a crash.
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	w, db := open()
	w.Do(func(tx *world.Tx) {
		tx.Block(cube.PosFromVec3(pos))
	})
	w.Save()
	col, err := db.LoadColumn(world.ChunkPos{}, world.Overworld)
	if err != nil {
		t.Fatal(err)
	}
	if len(col.Entities) != 0 {
		t.Fatalf("saved player record count before rejoin = %d, want 0", len(col.Entities))
	}
	w.Do(join)
	w.Save()
	col, err = db.LoadColumn(world.ChunkPos{}, world.Overworld)
	if err != nil {
		t.Fatal(err)
	}
	if len(col.Entities) != 1 {
		t.Fatalf("saved player record count after rejoin = %d, want 1", len(col.Entities))
	}
	if got, want := col.Entities[0].ID, int64(binary.LittleEndian.Uint64(id[8:])); got != want {
		t.Fatalf("saved player ID = %d, want %d", got, want)
	}
	w.Do(func(tx *world.Tx) {
		for p := range tx.Players() {
			if err := p.Close(); err != nil {
				t.Fatal(err)
			}
		}
	})
	w.Save()
	col, err = db.LoadColumn(world.ChunkPos{}, world.Overworld)
	if err != nil {
		t.Fatal(err)
	}
	if len(col.Entities) != 0 {
		t.Fatalf("saved player record count after disconnect = %d, want 0", len(col.Entities))
	}
}
