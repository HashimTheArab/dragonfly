package world

import (
	"log/slog"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
)

type benchmarkBlock struct{}

func (benchmarkBlock) EncodeBlock() (string, map[string]any) {
	return "minecraft:stone", nil
}

func (benchmarkBlock) Hash() (uint64, uint64) {
	return 1, 0
}

func (benchmarkBlock) Model() BlockModel {
	return unknownModel{}
}

type repeatedBlockStructure struct {
	dim [3]int
	b   Block
}

func (s repeatedBlockStructure) Dimensions() [3]int {
	return s.dim
}

func (s repeatedBlockStructure) At(_, _, _ int, _ func(x, y, z int) Block) (Block, Liquid) {
	return s.b, nil
}

type uniformBlockStructure struct {
	dim [3]int
	b   Block
}

func (s uniformBlockStructure) Dimensions() [3]int {
	return s.dim
}

func (s uniformBlockStructure) At(_, _, _ int, _ func(x, y, z int) Block) (Block, Liquid) {
	return s.b, nil
}

func (s uniformBlockStructure) Uniform() (Block, Liquid, bool) {
	return s.b, nil, true
}

type repeatedRuntimeIDStructure struct {
	dim [3]int
	rid uint32
	b   Block
}

func (s repeatedRuntimeIDStructure) Dimensions() [3]int {
	return s.dim
}

func (s repeatedRuntimeIDStructure) At(_, _, _ int, _ func(x, y, z int) Block) (Block, Liquid) {
	return s.b, nil
}

func (s repeatedRuntimeIDStructure) RuntimeIDAt(_, _, _ int, _ func(x, y, z int) Block) RuntimeIDPlacement {
	return RuntimeIDPlacement{BlockRuntimeID: s.rid, Block: s.b, PlaceBlock: true}
}

type paletteRuntimeIDStructure struct {
	dim  [3]int
	rids []uint32
}

func (s paletteRuntimeIDStructure) Dimensions() [3]int {
	return s.dim
}

func (s paletteRuntimeIDStructure) At(x, y, z int, _ func(x, y, z int) Block) (Block, Liquid) {
	return DefaultBlockRegistry.BlockByRuntimeIDOrAir(s.rids[(x+y+z)&1]), nil
}

func (s paletteRuntimeIDStructure) RuntimeIDAt(x, y, z int, _ func(x, y, z int) Block) RuntimeIDPlacement {
	// No Block value is needed: neither palette entry is an NBT block.
	return RuntimeIDPlacement{BlockRuntimeID: s.rids[(x+y+z)&1], PlaceBlock: true}
}

type repeatedRIDBackedLegacyStructure struct {
	dim  [3]int
	rids []uint32
}

func (s repeatedRIDBackedLegacyStructure) Dimensions() [3]int {
	return s.dim
}

func (s repeatedRIDBackedLegacyStructure) At(x, y, z int, _ func(x, y, z int) Block) (Block, Liquid) {
	return DefaultBlockRegistry.BlockByRuntimeIDOrAir(s.rids[(x+y+z)&1]), nil
}

func benchmarkRegisteredStone() Block {
	if !DefaultBlockRegistry.Finalized() {
		if _, ok := DefaultBlockRegistry.BlockByName("minecraft:stone", nil); ok {
			if !DefaultBlockRegistry.BlockImplemented("minecraft:stone", nil) {
				RegisterBlock(benchmarkBlock{})
			}
		}
		DefaultBlockRegistry.Finalize()
	}
	b, _ := BlockByName("minecraft:stone", nil)
	return b
}

func BenchmarkWorldBuildStructureRepeatedBlock(b *testing.B) {
	w := newBenchmarkWorld()
	s := repeatedBlockStructure{dim: [3]int{64, 64, 64}, b: benchmarkRegisteredStone()}
	pos := cube.Pos{0, 0, 0}

	w.buildStructure(pos, s)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.buildStructure(pos, s)
	}
}

func BenchmarkWorldBuildStructureUniformRepeatedBlock(b *testing.B) {
	w := newBenchmarkWorld()
	s := uniformBlockStructure{dim: [3]int{64, 64, 64}, b: benchmarkRegisteredStone()}
	pos := cube.Pos{0, 0, 0}

	w.buildStructure(pos, s)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.buildStructure(pos, s)
	}
}

func BenchmarkWorldBuildStructureRuntimeIDRepeatedBlock(b *testing.B) {
	w := newBenchmarkWorld()
	stone := benchmarkRegisteredStone()
	s := repeatedRuntimeIDStructure{dim: [3]int{64, 64, 64}, rid: BlockRuntimeID(stone), b: stone}
	pos := cube.Pos{0, 0, 0}

	w.buildStructure(pos, s)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.buildStructure(pos, s)
	}
}

func BenchmarkWorldBuildStructureRuntimeIDBackedPalette(b *testing.B) {
	w := newBenchmarkWorld()
	s := paletteRuntimeIDStructure{
		dim:  [3]int{64, 64, 64},
		rids: []uint32{BlockRuntimeID(benchmarkRegisteredStone()), BlockRuntimeID(benchmarkDirtBlock{})},
	}
	pos := cube.Pos{0, 0, 0}

	w.buildStructure(pos, s)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.buildStructure(pos, s)
	}
}

func BenchmarkWorldBuildStructureLegacyRIDBackedPalette(b *testing.B) {
	w := newBenchmarkWorld()
	s := repeatedRIDBackedLegacyStructure{
		dim:  [3]int{64, 64, 64},
		rids: []uint32{BlockRuntimeID(benchmarkRegisteredStone()), BlockRuntimeID(benchmarkDirtBlock{})},
	}
	pos := cube.Pos{0, 0, 0}

	w.buildStructure(pos, s)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.buildStructure(pos, s)
	}
}

func BenchmarkWorldFillVolumeRepeatedBlock(b *testing.B) {
	w := newBenchmarkWorld()
	pos := cube.Pos{0, 0, 0}
	dims := [3]int{64, 64, 64}
	stone := benchmarkRegisteredStone()

	w.fillVolume(pos, dims, stone, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.fillVolume(pos, dims, stone, nil)
	}
}

func newBenchmarkWorld() *World {
	DefaultBlockRegistry.Finalize()
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
