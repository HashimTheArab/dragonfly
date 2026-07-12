package world

import (
	"context"
	"log/slog"
	"reflect"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

type benchmarkNBTBlock struct {
	name string
}

func (benchmarkNBTBlock) EncodeBlock() (string, map[string]any) {
	return "minecraft:chest", map[string]any{"minecraft:cardinal_direction": "north"}
}

func (benchmarkNBTBlock) Hash() (uint64, uint64) {
	return 2, 0
}

func (benchmarkNBTBlock) Model() BlockModel {
	return unknownModel{}
}

func (b benchmarkNBTBlock) EncodeNBT() map[string]any {
	return map[string]any{"id": "Chest", "CustomName": b.name}
}

func (b benchmarkNBTBlock) DecodeNBT(data map[string]any) any {
	if name, ok := data["CustomName"].(string); ok {
		b.name = name
	}
	return b
}

type benchmarkLiquid struct{}

func (benchmarkLiquid) EncodeBlock() (string, map[string]any) {
	return "minecraft:water", map[string]any{"liquid_depth": int32(0)}
}

func (benchmarkLiquid) Hash() (uint64, uint64) {
	return 3, 0
}

func (benchmarkLiquid) Model() BlockModel {
	return unknownModel{}
}

func (benchmarkLiquid) LiquidDepth() int {
	return 8
}

func (benchmarkLiquid) SpreadDecay() int {
	return 1
}

func (benchmarkLiquid) WithDepth(int, bool) Liquid {
	return benchmarkLiquid{}
}

func (benchmarkLiquid) LiquidFalling() bool {
	return false
}

func (benchmarkLiquid) LiquidType() string {
	return "water"
}

func (benchmarkLiquid) Harden(cube.Pos, *Tx, *cube.Pos) bool {
	return false
}

func (benchmarkLiquid) LiquidRemoveBlock(cube.Pos, *Tx, Block) {}

func (benchmarkLiquid) BlastResistance() float64 {
	return 500
}

type unregisteredBlock struct{}

func (unregisteredBlock) EncodeBlock() (string, map[string]any) {
	return "minecraft:not_registered", nil
}

func (unregisteredBlock) Hash() (uint64, uint64) {
	return 4, 0
}

func (unregisteredBlock) Model() BlockModel {
	return unknownModel{}
}

type benchmarkDirtBlock struct{}

func (benchmarkDirtBlock) EncodeBlock() (string, map[string]any) {
	return "minecraft:dirt", nil
}

func (benchmarkDirtBlock) Hash() (uint64, uint64) {
	return 5, 0
}

func (benchmarkDirtBlock) Model() BlockModel {
	return unknownModel{}
}

func init() {
	if !DefaultBlockRegistry.Finalized() {
		RegisterBlock(benchmarkBlock{})
		RegisterBlock(benchmarkDirtBlock{})
		RegisterBlock(benchmarkNBTBlock{})
		RegisterBlock(benchmarkLiquid{})
		DefaultBlockRegistry.Finalize()
	}
}

func newTestWorld() *World {
	return &World{
		conf: Config{
			Log:             slog.Default(),
			Dim:             Overworld,
			Provider:        NopProvider{},
			Generator:       NopGenerator{},
			Blocks:          DefaultBlockRegistry,
			DisableLighting: true,
		},
		ra:       Overworld.Range(),
		chunks:   make(map[ChunkPos]*Column),
		entities: make(map[*EntityHandle]ChunkPos),
		viewers:  make(map[*Loader]Viewer),
	}
}

func TestFillVolumeClipsYBounds(t *testing.T) {
	w := newTestWorld()
	stone := benchmarkRegisteredStone()
	minY := w.Range()[0]

	w.fillVolume(cube.Pos{0, minY - 2, 0}, [3]int{2, 4, 2}, stone, nil)

	c := w.chunk(ChunkPos{0, 0})
	rid := BlockRuntimeID(stone)
	if got := c.Block(0, int16(minY), 0, 0); got != rid {
		t.Fatalf("expected clipped fill at min Y to have runtime ID %v, got %v", rid, got)
	}
	if got := c.Block(0, int16(minY+2), 0, 0); got != DefaultBlockRegistry.AirRuntimeID() {
		t.Fatalf("expected block above clipped volume to remain air, got runtime ID %v", got)
	}
}

func TestFillVolumeOutOfRangeDoesNotResolveBlocks(t *testing.T) {
	w := newTestWorld()

	w.fillVolume(cube.Pos{0, w.Range()[1] + 1, 0}, [3]int{1, 1, 1}, unregisteredBlock{}, nil)

	if len(w.chunks) != 0 {
		t.Fatalf("expected out-of-range fill to load no chunks, loaded %v", len(w.chunks))
	}
}

func TestFillVolumeBlockEntityCleanup(t *testing.T) {
	w := newTestWorld()
	pos := cube.Pos{0, 0, 0}

	w.fillVolume(pos, [3]int{1, 1, 1}, benchmarkNBTBlock{name: "test"}, nil)

	c := w.chunk(ChunkPos{0, 0})
	if _, ok := c.BlockEntities[pos]; !ok {
		t.Fatal("expected NBT block fill to create a block entity")
	}

	w.fillVolume(pos, [3]int{1, 1, 1}, benchmarkRegisteredStone(), nil)
	if _, ok := c.BlockEntities[pos]; ok {
		t.Fatal("expected non-NBT fill to delete stale block entity")
	}
}

func TestFillVolumeLiquidWriteAndClear(t *testing.T) {
	w := newTestWorld()
	pos := cube.Pos{0, 0, 0}
	stone := benchmarkRegisteredStone()
	water := benchmarkLiquid{}

	w.fillVolume(pos, [3]int{2, 1, 1}, stone, water)

	c := w.chunk(ChunkPos{0, 0})
	waterRID := BlockRuntimeID(water)
	for x := uint8(0); x < 2; x++ {
		if got := c.Block(x, 0, 0, 1); got != waterRID {
			t.Fatalf("expected liquid layer at x=%v to be %v, got %v", x, waterRID, got)
		}
	}

	w.fillVolume(pos, [3]int{2, 1, 1}, stone, nil)
	for x := uint8(0); x < 2; x++ {
		if got := c.Block(x, 0, 0, 1); got != DefaultBlockRegistry.AirRuntimeID() {
			t.Fatalf("expected liquid layer at x=%v to be cleared, got %v", x, got)
		}
	}
}

type recordingViewer struct {
	NopViewer
	chunks []ChunkPos
}

func (v *recordingViewer) ViewChunk(pos ChunkPos, _ Dimension, _ map[cube.Pos]Block, _ *chunk.Chunk) {
	v.chunks = append(v.chunks, pos)
}

func TestFillVolumeMarksAndUpdatesAffectedChunks(t *testing.T) {
	w := newTestWorld()
	viewer := &recordingViewer{}
	c0 := w.chunk(ChunkPos{0, 0})
	c1 := w.chunk(ChunkPos{1, 0})
	c0.viewers = []Viewer{viewer}
	c1.viewers = []Viewer{viewer}

	w.fillVolume(cube.Pos{15, 0, 0}, [3]int{2, 1, 1}, benchmarkRegisteredStone(), nil)

	if !c0.modified || !c1.modified {
		t.Fatal("expected all affected chunks to be marked modified")
	}
	if len(viewer.chunks) != 2 {
		t.Fatalf("expected one chunk update per affected chunk, got %v", len(viewer.chunks))
	}
}

type fastRuntimeIDStructure struct {
	dim          [3]int
	rid          uint32
	b            Block
	atCalls      int
	runtimeCalls int
}

func (s *fastRuntimeIDStructure) Dimensions() [3]int {
	return s.dim
}

func (s *fastRuntimeIDStructure) At(int, int, int, func(int, int, int) Block) (Block, Liquid) {
	s.atCalls++
	return nil, nil
}

func (s *fastRuntimeIDStructure) RuntimeIDAt(int, int, int, func(int, int, int) Block) RuntimeIDPlacement {
	s.runtimeCalls++
	return RuntimeIDPlacement{BlockRuntimeID: s.rid, Block: s.b, PlaceBlock: true}
}

func TestBuildStructureUsesRuntimeIDStructure(t *testing.T) {
	w := newTestWorld()
	stone := benchmarkRegisteredStone()
	rid := BlockRuntimeID(stone)
	s := &fastRuntimeIDStructure{dim: [3]int{1, 1, 1}, rid: rid, b: stone}

	w.buildStructure(cube.Pos{0, 0, 0}, s)

	if s.atCalls != 0 {
		t.Fatalf("expected RuntimeIDStructure path not to call At, got %v calls", s.atCalls)
	}
	if s.runtimeCalls != 1 {
		t.Fatalf("expected RuntimeIDAt to be called once, got %v", s.runtimeCalls)
	}
	if got := w.chunk(ChunkPos{0, 0}).Block(0, 0, 0, 0); got != rid {
		t.Fatalf("expected runtime ID %v to be placed, got %v", rid, got)
	}
}

type uniformCountingStructure struct {
	dim          [3]int
	b            Block
	liq          Liquid
	atCalls      int
	uniformCalls int
}

func (s *uniformCountingStructure) Dimensions() [3]int {
	return s.dim
}

func (s *uniformCountingStructure) At(int, int, int, func(int, int, int) Block) (Block, Liquid) {
	s.atCalls++
	return s.b, s.liq
}

func (s *uniformCountingStructure) Uniform() (Block, Liquid, bool) {
	s.uniformCalls++
	return s.b, s.liq, true
}

func TestBuildStructureUsesUniformStructure(t *testing.T) {
	w := newTestWorld()
	stone := benchmarkRegisteredStone()
	s := &uniformCountingStructure{dim: [3]int{18, 2, 2}, b: stone}

	w.buildStructure(cube.Pos{15, 0, 0}, s)

	if s.atCalls != 0 {
		t.Fatalf("expected UniformStructure path not to call At, got %v calls", s.atCalls)
	}
	if s.uniformCalls != 1 {
		t.Fatalf("expected Uniform to be called once, got %v", s.uniformCalls)
	}
	rid := BlockRuntimeID(stone)
	for _, tt := range []cube.Pos{{15, 0, 0}, {16, 1, 1}, {32, 0, 1}} {
		if got := w.chunk(chunkPosFromBlockPos(tt)).Block(uint8(tt[0]), int16(tt[1]), uint8(tt[2]), 0); got != rid {
			t.Fatalf("expected %v to have runtime ID %v, got %v", tt, rid, got)
		}
	}
}

func TestBuildStructureUniformStructureLiquidWriteAndClear(t *testing.T) {
	w := newTestWorld()
	pos := cube.Pos{0, 0, 0}
	stone := benchmarkRegisteredStone()
	water := benchmarkLiquid{}

	w.buildStructure(pos, &uniformCountingStructure{dim: [3]int{2, 1, 1}, b: stone, liq: water})

	c := w.chunk(ChunkPos{0, 0})
	waterRID := BlockRuntimeID(water)
	if got := c.Block(0, 0, 0, 1); got != waterRID {
		t.Fatalf("expected liquid layer to be %v, got %v", waterRID, got)
	}

	w.buildStructure(pos, &uniformCountingStructure{dim: [3]int{2, 1, 1}, b: stone})
	if got := c.Block(0, 0, 0, 1); got != DefaultBlockRegistry.AirRuntimeID() {
		t.Fatalf("expected liquid layer to be cleared, got %v", got)
	}
}

func TestBuildStructureRuntimeIDStructureInvalidRuntimeIDPanic(t *testing.T) {
	w := newTestWorld()
	s := &fastRuntimeIDStructure{dim: [3]int{1, 1, 1}, rid: uint32(DefaultBlockRegistry.BlockCount())}

	defer func() {
		if recover() == nil {
			t.Fatal("expected invalid runtime ID to panic")
		}
	}()
	w.buildStructure(cube.Pos{0, 0, 0}, s)
}

type alternatingBlockStructure struct {
	a, b Block
}

func (s alternatingBlockStructure) Dimensions() [3]int {
	return [3]int{2, 1, 1}
}

func (s alternatingBlockStructure) At(x, _, _ int, _ func(x, y, z int) Block) (Block, Liquid) {
	if x%2 == 0 {
		return s.a, nil
	}
	return s.b, nil
}

func TestBuildStructureRuntimeIDCachePlacesDistinctBlocks(t *testing.T) {
	w := newTestWorld()
	stone := benchmarkRegisteredStone()
	dirt := benchmarkDirtBlock{}

	w.buildStructure(cube.Pos{0, 0, 0}, alternatingBlockStructure{a: stone, b: dirt})

	c := w.chunk(ChunkPos{0, 0})
	if got, want := c.Block(0, 0, 0, 0), BlockRuntimeID(stone); got != want {
		t.Fatalf("expected first block runtime ID %v, got %v", want, got)
	}
	if got, want := c.Block(1, 0, 0, 0), BlockRuntimeID(dirt); got != want {
		t.Fatalf("expected second block runtime ID %v, got %v", want, got)
	}
}

func TestBuildStructureRuntimeIDStructureMatchesLegacyStructure(t *testing.T) {
	pos := cube.Pos{15, 15, 15}
	dim := [3]int{17, 17, 17}

	for _, tt := range []struct {
		name  string
		block Block
	}{
		{name: "stone", block: benchmarkRegisteredStone()},
		{name: "nbt", block: benchmarkNBTBlock{name: "parity"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			legacyWorld := newTestWorld()
			legacyWorld.buildStructure(pos, repeatedBlockStructure{dim: dim, b: tt.block})

			runtimeIDWorld := newTestWorld()
			runtimeIDWorld.buildStructure(pos, repeatedRuntimeIDStructure{dim: dim, rid: BlockRuntimeID(tt.block), b: tt.block})

			for _, chunkPos := range []ChunkPos{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				legacyChunk := legacyWorld.chunk(chunkPos)
				runtimeIDChunk := runtimeIDWorld.chunk(chunkPos)
				if !legacyChunk.Equals(runtimeIDChunk.Chunk) {
					t.Fatalf("runtime ID structure produced different chunk data at %v", chunkPos)
				}
				if !reflect.DeepEqual(legacyChunk.BlockEntities, runtimeIDChunk.BlockEntities) {
					t.Fatalf("runtime ID structure produced different block entities at %v", chunkPos)
				}
			}
		})
	}
}

func TestTxFillVolumeThroughWorldDo(t *testing.T) {
	w := Config{DisableLighting: true}.New()
	defer w.Close()

	if err := w.Do(func(tx *Tx) {
		tx.FillVolume(cube.Pos{0, 0, 0}, [3]int{1, 1, 1}, benchmarkRegisteredStone(), nil)
	}).Wait(context.Background()); err != nil {
		t.Fatalf("fill volume task failed: %v", err)
	}

	var got Block
	if err := w.Do(func(tx *Tx) {
		got = tx.Block(cube.Pos{0, 0, 0})
	}).Wait(context.Background()); err != nil {
		t.Fatalf("read block task failed: %v", err)
	}
	if gotRID, wantRID := BlockRuntimeID(got), BlockRuntimeID(benchmarkRegisteredStone()); gotRID != wantRID {
		t.Fatalf("expected runtime ID %v, got %v", wantRID, gotRID)
	}
}

func TestTxFillVolumeConcurrentDo(t *testing.T) {
	// Do serializes transactions, so this test exercises the Do
	// submission/completion path under -race rather than concurrent fill
	// bodies.
	w := Config{DisableLighting: true}.New()
	defer w.Close()

	const fills = 8
	done := make(chan struct{}, fills)
	for i := 0; i < fills; i++ {
		i := i
		go func() {
			err := w.Do(func(tx *Tx) {
				tx.FillVolume(cube.Pos{i, 0, 0}, [3]int{4, 4, 4}, benchmarkRegisteredStone(), nil)
			}).Wait(context.Background())
			if err != nil {
				t.Errorf("fill volume task %d failed: %v", i, err)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < fills; i++ {
		<-done
	}

	if err := w.Do(func(tx *Tx) {
		wantRID := BlockRuntimeID(benchmarkRegisteredStone())
		for i := 0; i < fills; i++ {
			if gotRID := BlockRuntimeID(tx.Block(cube.Pos{i, 0, 0})); gotRID != wantRID {
				t.Fatalf("expected fill at x=%v to have runtime ID %v, got %v", i, wantRID, gotRID)
			}
		}
	}).Wait(context.Background()); err != nil {
		t.Fatalf("verify fills task failed: %v", err)
	}
}
