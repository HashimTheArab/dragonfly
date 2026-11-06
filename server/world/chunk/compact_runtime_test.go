package chunk

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
)

func TestPalettedStorageCompactForRuntimeCacheCollapsesSingleValueStorage(t *testing.T) {
	storage := newPalettedStorage(make([]uint32, paletteSize(1).uint32s()), newPalette(1, []uint32{42}))

	storage.compactForRuntimeCache()

	if storage.bitsPerIndex != 0 {
		t.Fatalf("bitsPerIndex = %d, want 0", storage.bitsPerIndex)
	}
	if storage.indices != nil {
		t.Fatalf("indices = %v, want nil", storage.indices)
	}
	if storage.indicesStart != nil {
		t.Fatalf("indicesStart = %v, want nil", storage.indicesStart)
	}
	if got := storage.At(3, 4, 5); got != 42 {
		t.Fatalf("At() = %d, want retained single palette value 42", got)
	}
}

func TestSubChunkCompactForRuntimeCacheDropsSingleValueAirStorage(t *testing.T) {
	sub := NewSubChunk(7)
	sub.storages = []*PalettedStorage{
		newPalettedStorage(make([]uint32, paletteSize(1).uint32s()), newPalette(1, []uint32{7})),
	}

	sub.compactForRuntimeCache()

	if len(sub.storages) != 0 {
		t.Fatalf("len(storages) = %d, want 0", len(sub.storages))
	}
}

func TestSubChunkCompactForRuntimeCacheDropsZeroIndexedAirStorage(t *testing.T) {
	sub := NewSubChunk(7)
	sub.storages = []*PalettedStorage{
		newPalettedStorage(make([]uint32, paletteSize(4).uint32s()), newPalette(4, []uint32{7, 11, 22})),
	}

	sub.compactForRuntimeCache()

	if len(sub.storages) != 0 {
		t.Fatalf("len(storages) = %d, want 0", len(sub.storages))
	}
}

func TestPalettedStorageCompactForRuntimeCacheCollapsesZeroIndexedStorage(t *testing.T) {
	storage := newPalettedStorage(make([]uint32, paletteSize(4).uint32s()), newPalette(4, []uint32{42, 11, 22}))

	storage.compactForRuntimeCache()

	if storage.bitsPerIndex != 0 {
		t.Fatalf("bitsPerIndex = %d, want 0", storage.bitsPerIndex)
	}
	if storage.indices != nil {
		t.Fatalf("indices = %v, want nil", storage.indices)
	}
	if got := storage.palette.Len(); got != 1 {
		t.Fatalf("palette len = %d, want 1", got)
	}
	if got := storage.At(3, 4, 5); got != 42 {
		t.Fatalf("At() = %d, want retained palette index 0 value 42", got)
	}
}

func TestPalettedStorageCompactForRuntimeCacheCollapsesUniformNonZeroIndexStorage(t *testing.T) {
	storage := newPalettedStorage(make([]uint32, paletteSize(4).uint32s()), newPalette(4, []uint32{11, 42, 22}))
	fillStorage(storage, 42)

	storage.compactForRuntimeCache()

	if storage.bitsPerIndex != 0 {
		t.Fatalf("bitsPerIndex = %d, want 0", storage.bitsPerIndex)
	}
	if got := storage.palette.Len(); got != 1 {
		t.Fatalf("palette len = %d, want 1", got)
	}
	if got := storage.At(3, 4, 5); got != 42 {
		t.Fatalf("At() = %d, want retained uniform value 42", got)
	}
}

func TestSubChunkCompactForRuntimeCacheDropsUniformNonZeroIndexAirStorage(t *testing.T) {
	sub := NewSubChunk(7)
	storage := newPalettedStorage(make([]uint32, paletteSize(4).uint32s()), newPalette(4, []uint32{11, 7, 22}))
	fillStorage(storage, 7)
	sub.storages = []*PalettedStorage{storage}

	sub.compactForRuntimeCache()

	if len(sub.storages) != 0 {
		t.Fatalf("len(storages) = %d, want 0", len(sub.storages))
	}
}

func TestChunkCompactForRuntimeCacheDoesNotRepackMultiValueStorage(t *testing.T) {
	c := New(testBlockRegistry{air: 0}, testRange())
	storage := newPalettedStorage(make([]uint32, paletteSize(1).uint32s()), newPalette(1, []uint32{1, 2}))
	storage.Set(1, 2, 3, 2)
	c.sub[0].storages = []*PalettedStorage{storage}
	indices := &storage.indices[0]
	paletteValues := len(storage.palette.values)

	c.CompactForRuntimeCache()

	if c.sub[0].storages[0] != storage {
		t.Fatal("multi-value storage pointer changed; cheap compaction should not repack it")
	}
	if &storage.indices[0] != indices {
		t.Fatal("multi-value storage indices were replaced; cheap compaction should not scan/rewrite them")
	}
	if got := len(storage.palette.values); got != paletteValues {
		t.Fatalf("palette len = %d, want %d", got, paletteValues)
	}
}

func TestPalettedStorageCompactForRuntimeCacheShrinksOversizedMultiValueStorage(t *testing.T) {
	storage := newPalettedStorage(make([]uint32, paletteSize(4).uint32s()), newPalette(4, []uint32{11, 22}))
	storage.Set(1, 2, 3, 22)

	storage.compactForRuntimeCache()

	if got, want := storage.bitsPerIndex, uint16(1); got != want {
		t.Fatalf("bitsPerIndex = %d, want %d", got, want)
	}
	if got, want := len(storage.indices), paletteSize(1).uint32s(); got != want {
		t.Fatalf("len(indices) = %d, want %d", got, want)
	}
	if got := len(storage.palette.values); got != 2 {
		t.Fatalf("palette len = %d, want 2", got)
	}
	if got := storage.At(1, 2, 3); got != 22 {
		t.Fatalf("At(1,2,3) = %d, want 22", got)
	}
}

// TestPalettedStorageCompactForRuntimeCacheResolvesUncoveredIndex checks that shrinking a decoded
// storage holding an index its palette does not cover leaves no such index behind. It would not
// fit the narrower width, and a value added to the palette later would change what the cell reads.
func TestPalettedStorageCompactForRuntimeCacheResolvesUncoveredIndex(t *testing.T) {
	storage := newPalettedStorage(make([]uint32, paletteSize(4).uint32s()), newPalette(4, []uint32{11, 22}))
	storage.Set(1, 2, 3, 22)
	storage.setPaletteIndex(1, 15, 0, 5)

	storage.compactForRuntimeCache()

	if got, want := storage.bitsPerIndex, uint16(1); got != want {
		t.Fatalf("bitsPerIndex = %d, want %d", got, want)
	}
	for x := byte(0); x < 16; x++ {
		for y := byte(0); y < 16; y++ {
			for z := byte(0); z < 16; z++ {
				if got := storage.PaletteIndex(x, y, z); int(got) >= storage.palette.Len() {
					t.Fatalf("PaletteIndex(%d,%d,%d) = %d, outside a palette of %d", x, y, z, got, storage.palette.Len())
				}
			}
		}
	}
	// Enough new values to reach index 5, on cells away from the ones checked below.
	for v := uint32(33); v <= 66; v += 11 {
		storage.Set(9, 0, 0, v)
	}
	if got := storage.At(1, 15, 0); got != 11 {
		t.Fatalf("At(1,15,0) = %d, want the fallback value 11", got)
	}
	if got := storage.At(1, 1, 1); got != 11 {
		t.Fatalf("At(1,1,1) = %d, want 11", got)
	}
	if got := storage.At(1, 2, 3); got != 22 {
		t.Fatalf("At(1,2,3) = %d, want 22", got)
	}
}

// TestPalettedStorageResizeKeepsPaletteSizeInStep checks that resize alone keeps the palette
// size equal to the index width, whichever way it moves the width.
func TestPalettedStorageResizeKeepsPaletteSizeInStep(t *testing.T) {
	storage := newPalettedStorage(make([]uint32, paletteSize(4).uint32s()), newPalette(4, []uint32{11, 22}))

	for _, size := range []paletteSize{1, 2, 1} {
		storage.resize(size)
		if got := storage.palette.size; got != size {
			t.Fatalf("palette size = %d after resizing to %d", got, size)
		}
		if got := storage.bitsPerIndex; got != uint16(size) {
			t.Fatalf("bitsPerIndex = %d after resizing to %d", got, size)
		}
	}
}

type testBlockRegistry struct {
	air uint32
}

func (r testBlockRegistry) BlockCount() int { return 3 }
func (r testBlockRegistry) AirRuntimeID() uint32 {
	return r.air
}
func (testBlockRegistry) RuntimeIDToState(runtimeID uint32) (string, map[string]any, bool) {
	return "test:block", nil, true
}
func (testBlockRegistry) StateToRuntimeID(string, map[string]any) (uint32, bool) { return 0, true }
func (testBlockRegistry) DefaultProperties(string) (map[string]any, bool)        { return nil, false }
func (testBlockRegistry) FilteringBlock(uint32) uint8                            { return 0 }
func (testBlockRegistry) LightBlock(uint32) uint8                                { return 0 }
func (testBlockRegistry) RandomTickBlock(uint32) bool                            { return false }
func (testBlockRegistry) NBTBlock(uint32) bool                                   { return false }
func (testBlockRegistry) LiquidDisplacingBlock(uint32) bool                      { return false }
func (testBlockRegistry) LiquidBlock(uint32) bool                                { return false }
func (testBlockRegistry) HashToRuntimeID(hash uint32) (uint32, bool)             { return hash, true }
func (testBlockRegistry) RuntimeIDToHash(runtimeID uint32) (uint32, bool)        { return runtimeID, true }

func testRange() cube.Range {
	return cube.Range{0, 15}
}

func fillStorage(storage *PalettedStorage, runtimeID uint32) {
	for x := byte(0); x < 16; x++ {
		for y := byte(0); y < 16; y++ {
			for z := byte(0); z < 16; z++ {
				storage.Set(x, y, z, runtimeID)
			}
		}
	}
}

// TestPalettedStorageCompactForRuntimeCacheGrowsAgainAfterShrinking checks that a storage
// shrunk by the cheap compaction widens again as values are added. Servers such as Geyser
// encode storages wider than their palettes need, and an index must never overflow into
// the next block.
func TestPalettedStorageCompactForRuntimeCacheGrowsAgainAfterShrinking(t *testing.T) {
	storage := newPalettedStorage(make([]uint32, paletteSize(4).uint32s()), newPalette(4, []uint32{11, 22}))
	storage.Set(1, 2, 3, 22)

	storage.compactForRuntimeCache()
	for v := uint32(33); v <= 99; v += 11 {
		storage.Set(1, 4, 3, v)
		if got := storage.At(1, 4, 3); got != v {
			t.Fatalf("At(1,4,3) = %d after setting %d", got, v)
		}
		if got := storage.At(1, 5, 3); got != 11 {
			t.Fatalf("At(1,5,3) = %d after setting %d below it, want 11", got, v)
		}
	}
	if got := storage.At(1, 2, 3); got != 22 {
		t.Fatalf("At(1,2,3) = %d, want 22", got)
	}
}
