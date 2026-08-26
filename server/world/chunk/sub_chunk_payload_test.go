package chunk_test

import (
	"slices"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type countingFilteringRegistry struct {
	chunk.BlockRegistry
	calls int
}

// FilteringBlock records how many blocks a height lookup examines.
func (r *countingFilteringRegistry) FilteringBlock(runtimeID uint32) uint8 {
	r.calls++
	return r.BlockRegistry.FilteringBlock(runtimeID)
}

// The client lights a sub-chunk from this map, so a surface lying wholly outside the
// sub-chunk has to collapse to TooHigh/TooLow rather than ship 256 sentinel heights.
func TestSubChunkHeightMap(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	c := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	stone, ok := world.DefaultBlockRegistry.StateToRuntimeID("minecraft:stone", map[string]any{"stone_type": "stone"})
	if !ok {
		stone = world.DefaultBlockRegistry.BlockRuntimeID(block.Stone{})
	}
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			c.SetBlock(x, 64, z, 0, stone)
		}
	}
	surface := c.SubIndex(64)

	t.Run("surface above the sub-chunk", func(t *testing.T) {
		mapType, heights := chunk.SubChunkHeightMap(c, surface-1)
		if mapType != protocol.HeightMapDataTooHigh {
			t.Fatalf("type = %d, want TooHigh", mapType)
		}
		if heights != nil {
			t.Errorf("carried %d heights, want none", len(heights))
		}
	})

	t.Run("surface below the sub-chunk", func(t *testing.T) {
		mapType, heights := chunk.SubChunkHeightMap(c, surface+1)
		if mapType != protocol.HeightMapDataTooLow {
			t.Fatalf("type = %d, want TooLow", mapType)
		}
		if heights != nil {
			t.Errorf("carried %d heights, want none", len(heights))
		}
	})

	t.Run("surface inside the sub-chunk", func(t *testing.T) {
		mapType, heights := chunk.SubChunkHeightMap(c, surface)
		if mapType != protocol.HeightMapDataHasData {
			t.Fatalf("type = %d, want HasData", mapType)
		}
		if len(heights) != 256 {
			t.Fatalf("carried %d heights, want 256", len(heights))
		}
		// The height map holds the first free Y above the surface, expressed relative to
		// the sub-chunk's own base rather than to world Y.
		want := int8(65 - c.SubY(surface))
		for i, h := range heights {
			if h != want {
				t.Fatalf("height[%d] = %d, want %d", i, h, want)
			}
		}
	})
}

// A reusable map must preserve the per-entry protocol contract while avoiding a
// fresh chunk-height lookup for every vertical offset in one response.
func TestSubChunkHeightMaps(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	c := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	stone := world.DefaultBlockRegistry.BlockRuntimeID(block.Stone{})
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			c.SetBlock(x, int16(48+int(x&3)*16), z, 0, stone)
		}
	}
	maps := chunk.NewSubChunkHeightMaps(c)
	for index := int16(0); index < int16(len(c.Sub())); index++ {
		gotType, gotHeights := maps.At(index)
		wantType, wantHeights := chunk.SubChunkHeightMap(c, index)
		if gotType != wantType {
			t.Fatalf("index %d type = %d, want %d", index, gotType, wantType)
		}
		if !slices.Equal(gotHeights, wantHeights) {
			t.Fatalf("index %d heights differ", index)
		}
	}
}

// LightBlockerMap exposes the cached chunk-wide scan using the exact semantics
// of HighestLightBlocker, including invalidation after a block edit.
func TestLightBlockerMap(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	c := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	stone := world.DefaultBlockRegistry.BlockRuntimeID(block.Stone{})
	c.SetBlock(2, 64, 3, 0, stone)
	heights := c.LightBlockerMap()

	if got := heights.At(2, 3); got != c.HighestLightBlocker(2, 3) {
		t.Fatalf("cached blocker = %d, want direct blocker %d", got, c.HighestLightBlocker(2, 3))
	}
	c.SetBlock(2, 96, 3, 0, stone)
	if got := heights.At(2, 3); got != 96 {
		t.Fatalf("cached blocker after edit = %d, want 96", got)
	}
	if got := heights.At(0, 0); got != int16(c.Range().Min()) {
		t.Fatalf("empty-column blocker = %d, want world minimum %d", got, c.Range().Min())
	}
}

// A skirt needs all centre columns but only one edge from each neighbour, so
// preparing one edge must not eagerly scan the neighbour's other 240 columns.
func TestLightBlockerMapComputesColumnsLazily(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	registry := &countingFilteringRegistry{BlockRegistry: world.DefaultBlockRegistry}
	c := chunk.New(registry, world.Overworld.Range())
	stone := world.DefaultBlockRegistry.BlockRuntimeID(block.Stone{})
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			c.SetBlock(x, 64, z, 0, stone)
		}
	}
	heights := c.LightBlockerMap()
	if got := heights.At(0, 0); got != 64 {
		t.Fatalf("first blocker = %d, want 64", got)
	}
	if registry.calls > 16 {
		t.Fatalf("first column examined %d blocks, want a lazy single-column lookup", registry.calls)
	}
	firstCalls := registry.calls
	if got := heights.At(0, 0); got != 64 {
		t.Fatalf("cached blocker = %d, want 64", got)
	}
	if registry.calls != firstCalls {
		t.Fatalf("cached column performed %d additional filtering lookups", registry.calls-firstCalls)
	}
}

// Chunk block coordinates may carry the low byte of an absolute position; the
// height cache must use the same four-bit local coordinates as block storage.
func TestLightBlockerMapMasksBlockCoordinates(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	c := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	stone := world.DefaultBlockRegistry.BlockRuntimeID(block.Stone{})
	c.SetBlock(0xff, 64, 0x50, 0, stone)
	if got := c.LightBlockerMap().At(0x0f, 0); got != 64 {
		t.Fatalf("masked blocker = %d, want 64", got)
	}
}

// BenchmarkSubChunkHeightMaps compares one prepared response summary with the
// compatibility helper that prepares each entry independently.
func BenchmarkSubChunkHeightMaps(b *testing.B) {
	world.DefaultBlockRegistry.Finalize()
	c := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	stone := world.DefaultBlockRegistry.BlockRuntimeID(block.Stone{})
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			c.SetBlock(x, int16(48+int(x&3)*16), z, 0, stone)
		}
	}
	indices := [...]int16{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	b.Run("individual", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for _, index := range indices {
				_, _ = chunk.SubChunkHeightMap(c, index)
			}
		}
	})
	b.Run("prepared", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			maps := chunk.NewSubChunkHeightMaps(c)
			for _, index := range indices {
				_, _ = maps.At(index)
			}
		}
	})
}

// Protocol 2168 constrains SubChunkCount to 0..64, so announcing with the old MaxUint32
// sentinel makes the client reject the column before it requests any sub-chunk.
func TestRequestModeLevelChunkUsesZeroCount(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	ch := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	count, limit := chunk.RequestModeLevelChunk(ch)
	if count != 0 {
		t.Errorf("SubChunkCount = %d, want 0", count)
	}
	got, ok := limit.Value()
	if !ok {
		t.Fatal("SubChunkLimit is unset, so the client requests nothing and the column stays empty")
	}
	if want := int32(ch.HighestFilledSubChunk()); got != want {
		t.Errorf("SubChunkLimit = %d, want %d", got, want)
	}
}
