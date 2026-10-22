package world

import (
	"errors"
	"testing"

	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/go-gl/mathgl/mgl64"
)

// entityLoadFailureProvider can fail destination loads while recording saves.
type entityLoadFailureProvider struct {
	columnSaveObserver
	fail bool
}

// LoadColumn fails the destination load until the provider recovers.
func (p *entityLoadFailureProvider) LoadColumn(pos ChunkPos, dim Dimension) (*chunk.Column, error) {
	if p.fail && pos == (ChunkPos{1, 0}) {
		return nil, errors.New("destination unavailable")
	}
	return p.NopProvider.LoadColumn(pos, dim)
}

func TestEntitySaveRetainsMembershipWhenDestinationLoadFails(t *testing.T) {
	p := &entityLoadFailureProvider{
		columnSaveObserver: columnSaveObserver{stored: make(map[ChunkPos]*chunk.Column)},
		fail:               true,
	}
	w := Config{Synchronous: true, Provider: p}.New()
	defer w.Close()
	h := EntitySpawnOpts{Position: mgl64.Vec3{1, 10, 1}}.New(testEntityType{}, testEntityConfig{})
	w.Do(func(tx *Tx) {
		tx.AddEntity(h)
		h.data.Pos = mgl64.Vec3{20, 10, 1}
	})
	w.Save()
	if col := p.stored[ChunkPos{}]; col == nil || len(col.Entities) != 1 {
		t.Fatal("failed destination load removed the entity from its saved source column")
	}
	p.fail = false
	w.Save()
	if col := p.stored[ChunkPos{1, 0}]; col == nil || len(col.Entities) != 1 {
		t.Fatal("entity was not saved in the destination after the provider recovered")
	}
	if col := p.stored[ChunkPos{}]; col == nil || len(col.Entities) != 0 {
		t.Fatal("entity remained in the source after the provider recovered")
	}
}

// persistencePlayerType gives the closing entity fixture a player's identity.
type persistencePlayerType struct{ taskTestEntityType }

// EncodeEntity identifies the fixture as a player for chunk ownership checks.
func (persistencePlayerType) EncodeEntity() string { return "minecraft:player" }

// Open binds the player fixture's position and close behavior to the transaction.
func (persistencePlayerType) Open(tx *Tx, h *EntityHandle, data *EntityData) Entity {
	return persistencePlayer{taskTestEntity: taskTestEntity{h: h, tx: tx}, data: data}
}

// persistencePlayer closes its handle like a player when its column is unloaded.
type persistencePlayer struct {
	taskTestEntity
	data *EntityData
}

// Position returns the player fixture's current position.
func (e persistencePlayer) Position() mgl64.Vec3 { return e.data.Pos }

func TestPlayerSurvivesUnloadBeforeDestinationViewer(t *testing.T) {
	for _, test := range []struct {
		name      string
		loaded    bool
		save      bool
		tick      bool
		companion bool
	}{
		{name: "unload/unloaded destination"},
		{name: "unload/loaded destination", loaded: true},
		{name: "save then unload/unloaded destination", save: true},
		{name: "save then unload/loaded destination", save: true, loaded: true},
		{name: "save then tick then unload/unloaded destination", save: true, tick: true},
		{name: "save then tick then unload/shared destination", save: true, tick: true, companion: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := Config{Synchronous: true}.New()
			defer w.Close()
			loader := NewLoader(0, w, NopViewer{})
			defer w.Do(func(tx *Tx) { loader.Close(tx) })
			source, destination := mgl64.Vec3{1, 10, 1}, mgl64.Vec3{20, 10, 1}
			h := EntitySpawnOpts{Position: source}.New(persistencePlayerType{}, taskTestEntityConfig{})
			w.Do(func(tx *Tx) {
				loader.Move(tx, source)
				loader.Load(tx, 1)
				tx.AddEntity(h)
				if test.loaded {
					tx.chunk(ChunkPos{1, 0})
				}
				h.data.Pos = destination
				if test.companion {
					other := EntitySpawnOpts{Position: source}.New(taskTestEntityType{}, taskTestEntityConfig{})
					tx.AddEntity(other)
					other.data.Pos = destination
				}
			})
			if test.save {
				w.Save()
			}
			if test.tick {
				w.AdvanceTick()
			}
			w.Do(func(tx *Tx) {
				w.closeUnusedChunks(tx)
				if pos, ok := w.entities[h]; !ok || h.Closed() || w.chunks[pos] == nil {
					t.Fatal("player was closed before the destination viewer registered")
				}
				loader.Move(tx, destination)
				loader.Load(tx, 1)
				w.closeUnusedChunks(tx)
				if pos, ok := w.entities[h]; !ok || pos != (ChunkPos{1, 0}) {
					t.Fatal("player did not move to its viewed destination")
				}
				if err := h.mustEntity(tx).Close(); err != nil {
					t.Fatal(err)
				}
				loader.Close(tx)
				w.closeUnusedChunks(tx)
				if w.chunks[ChunkPos{1, 0}] != nil {
					t.Fatal("destination stayed loaded after the player disconnected")
				}
			})
		})
	}
}
