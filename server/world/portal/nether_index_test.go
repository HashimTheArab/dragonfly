package portal_test

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/df-mc/dragonfly/server/world/portal"
	"github.com/df-mc/goleveldb/leveldb"
)

func TestFindNetherPortalSearchesChunkPalettes(t *testing.T) {
	registry := &countingBlockRegistry{BlockRegistry: world.DefaultBlockRegistry}
	provider := newPortalMemoryProvider()

	origin := cube.Pos{192, 10, 192}
	w := world.Config{Blocks: registry, Provider: provider}.New()
	<-w.Exec(func(tx *world.Tx) {
		buildVerticalFrame(tx, origin, cube.Z, 2, 3)
		if !portal.ActivateNetherPortal(tx, origin) {
			t.Fatal("ActivateNetherPortal() = false, want true")
		}
	})
	w.Save()
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	w = world.Config{Blocks: registry, Provider: provider}.New()
	t.Cleanup(func() { _ = w.Close() })
	<-w.Exec(func(tx *world.Tx) {
		registry.resetRuntimeLookupCalls()
		n, ok := portal.FindNetherPortal(tx, origin, 128)
		if !ok {
			t.Fatal("FindNetherPortal() = false, want true")
		}
		if n.Spawn() != origin {
			t.Fatalf("FindNetherPortal() spawn = %v, want %v", n.Spawn(), origin)
		}
		if calls := registry.blockByRuntimeIDCalls(); calls != 0 {
			t.Fatalf("FindNetherPortal() made %d direct runtime block lookups, want 0", calls)
		}
		if calls := registry.blockByRuntimeIDOrAirCalls(); calls > 10_000 {
			t.Fatalf("FindNetherPortal() made %d validation runtime block lookups, want at most 10,000", calls)
		}

		provider.resetLoadColumnCalls()
		if _, ok := portal.FindNetherPortal(tx, origin, 128); !ok {
			t.Fatal("FindNetherPortal() = false after indexing, want true")
		}
		if calls := provider.loadColumnCalls(); calls > 1 {
			t.Fatalf("FindNetherPortal() loaded %d columns after indexing, want at most 1", calls)
		}
	})
}

func TestFindNetherPortalDoesNotGenerateMissingChunks(t *testing.T) {
	generator := &countingGenerator{}
	w := world.Config{Provider: newPortalMemoryProvider(), Generator: generator}.New()
	t.Cleanup(func() { _ = w.Close() })

	<-w.Exec(func(tx *world.Tx) {
		if _, ok := portal.FindNetherPortal(tx, cube.Pos{192, 10, 192}, 128); ok {
			t.Fatal("FindNetherPortal() = true in empty provider, want false")
		}
		if calls := generator.generateCalls; calls != 0 {
			t.Fatalf("FindNetherPortal() generated %d chunks, want 0", calls)
		}
	})
}

func TestFindNetherPortalIgnoresRemovedPortalBlocks(t *testing.T) {
	w := world.New()
	t.Cleanup(func() { _ = w.Close() })

	origin := cube.Pos{8, 10, 8}
	<-w.Exec(func(tx *world.Tx) {
		buildVerticalFrame(tx, origin, cube.Z, 2, 3)
		if !portal.ActivateNetherPortal(tx, origin) {
			t.Fatal("ActivateNetherPortal() = false, want true")
		}
		n, ok := portal.FindNetherPortal(tx, origin, 16)
		if !ok {
			t.Fatal("FindNetherPortal() = false, want true")
		}
		for _, pos := range n.Positions() {
			tx.SetBlock(pos, nil, nil)
		}
		if _, ok := portal.FindNetherPortal(tx, origin, 16); ok {
			t.Fatal("FindNetherPortal() = true after removing portal blocks, want false")
		}
	})
}

type portalColumnKey struct {
	pos world.ChunkPos
	dim int
}

type portalMemoryProvider struct {
	world.NopProvider
	columns    map[portalColumnKey]*chunk.Column
	loadColumn int
}

func newPortalMemoryProvider() *portalMemoryProvider {
	return &portalMemoryProvider{
		NopProvider: world.NopProvider{Set: &world.Settings{Name: "World"}},
		columns:     make(map[portalColumnKey]*chunk.Column),
	}
}

func (p *portalMemoryProvider) LoadColumn(pos world.ChunkPos, dim world.Dimension) (*chunk.Column, error) {
	p.loadColumn++
	col, ok := p.columns[portalKey(pos, dim)]
	if !ok {
		return nil, leveldb.ErrNotFound
	}
	return col, nil
}

func (p *portalMemoryProvider) StoreColumn(pos world.ChunkPos, dim world.Dimension, col *chunk.Column) error {
	p.columns[portalKey(pos, dim)] = col
	return nil
}

func (p *portalMemoryProvider) resetLoadColumnCalls() {
	p.loadColumn = 0
}

func (p *portalMemoryProvider) loadColumnCalls() int {
	return p.loadColumn
}

func portalKey(pos world.ChunkPos, dim world.Dimension) portalColumnKey {
	id, ok := world.DimensionID(dim)
	if !ok {
		id = -1
	}
	return portalColumnKey{pos: pos, dim: id}
}

type countingBlockRegistry struct {
	world.BlockRegistry
	blockByRuntimeID      int
	blockByRuntimeIDOrAir int
}

func (r *countingBlockRegistry) BlockByRuntimeID(rid uint32) (world.Block, bool) {
	r.blockByRuntimeID++
	return r.BlockRegistry.BlockByRuntimeID(rid)
}

func (r *countingBlockRegistry) BlockByRuntimeIDOrAir(rid uint32) world.Block {
	r.blockByRuntimeIDOrAir++
	return r.BlockRegistry.BlockByRuntimeIDOrAir(rid)
}

func (r *countingBlockRegistry) resetRuntimeLookupCalls() {
	r.blockByRuntimeID = 0
	r.blockByRuntimeIDOrAir = 0
}

func (r *countingBlockRegistry) blockByRuntimeIDCalls() int {
	return r.blockByRuntimeID
}

func (r *countingBlockRegistry) blockByRuntimeIDOrAirCalls() int {
	return r.blockByRuntimeIDOrAir
}

type countingGenerator struct {
	generateCalls int
}

func (g *countingGenerator) GenerateChunk(world.ChunkPos, *chunk.Chunk) {
	g.generateCalls++
}

func (*countingGenerator) DefaultSpawn(world.Dimension) cube.Pos {
	return cube.Pos{}
}
