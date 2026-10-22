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
