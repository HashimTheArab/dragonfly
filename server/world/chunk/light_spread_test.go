package chunk

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
)

const (
	spreadAir uint32 = iota
	spreadStone
	spreadTorch
	spreadWater
)

// spreadTestRegistry has air, a solid block, a light source and a block that diffuses light like water.
type spreadTestRegistry struct{ testBlockRegistry }

func (spreadTestRegistry) BlockCount() int { return 4 }

func (spreadTestRegistry) FilteringBlock(rid uint32) uint8 {
	switch rid {
	case spreadStone:
		return 15
	case spreadWater:
		return 2
	}
	return 0
}

func (spreadTestRegistry) LightBlock(rid uint32) uint8 {
	if rid == spreadTorch {
		return 14
	}
	return 0
}

// newSpreadTestArea builds a 3x3 area of stone with a tunnel across all of it, a room holding water, and
// torches next to the borders between chunks, and returns its chunks and the torch positions.
func newSpreadTestArea() ([]*Chunk, []cube.Pos) {
	r := cube.Range{-16, 31}
	chunks := make([]*Chunk, 9)
	for i := range chunks {
		chunks[i] = New(spreadTestRegistry{}, r)
		for x := uint8(0); x < 16; x++ {
			for z := uint8(0); z < 16; z++ {
				for y := int16(r.Min()); y <= int16(r.Max()); y++ {
					chunks[i].SetBlock(x, y, z, 0, spreadStone)
				}
			}
		}
	}
	set := func(pos cube.Pos, rid uint32) {
		chunks[(pos[0]>>4)+(pos[2]>>4)*3].SetBlock(uint8(pos[0]&0xf), int16(pos[1]), uint8(pos[2]&0xf), 0, rid)
	}
	for x := 1; x < 47; x++ {
		set(cube.Pos{x, 4, 20}, spreadAir)
		set(cube.Pos{x, 5, 20}, spreadAir)
	}
	for x := 20; x < 28; x++ {
		for z := 21; z < 27; z++ {
			for y := 4; y < 8; y++ {
				set(cube.Pos{x, y, z}, spreadAir)
			}
			set(cube.Pos{x, 4, z}, spreadWater)
		}
	}
	torches := []cube.Pos{{15, 4, 20}, {33, 5, 20}, {24, 7, 25}}
	for _, pos := range torches {
		set(pos, spreadTorch)
	}
	return chunks, torches
}

// SpreadBlockLight lights exactly the positions, with exactly the levels, that the block light stage of a
// light area gives the same blocks, across chunk borders and through blocks that diffuse light.
func TestSpreadBlockLightMatchesLightArea(t *testing.T) {
	chunks, torches := newSpreadTestArea()
	at := func(x, z int) *Chunk {
		if x < 0 || z < 0 || x > 2 || z > 2 {
			return nil
		}
		return chunks[x+z*3]
	}
	got := SpreadBlockLight(at, torches, 1<<20)

	area := LightArea(chunks, 0, 0)
	area.Fill()
	area.Spread()
	r := chunks[0].Range()
	lit := 0
	for x := 0; x < 48; x++ {
		for z := 0; z < 48; z++ {
			for y := r.Min(); y <= r.Max(); y++ {
				pos := cube.Pos{x, y, z}
				want := area.light(pos, BlockLight)
				if got[pos] != want {
					t.Fatalf("block light at %v = %d, want %d", pos, got[pos], want)
				}
				if want > 0 {
					lit++
				}
			}
		}
	}
	if lit != len(got) {
		t.Fatalf("lit %d positions, want %d", len(got), lit)
	}
}

// SpreadBlockLight adds no more positions than its limit, and none in chunks that are not loaded.
func TestSpreadBlockLightStaysInLimitAndLoadedChunks(t *testing.T) {
	chunks, torches := newSpreadTestArea()
	onlyFirst := func(x, z int) *Chunk {
		if x == 0 && z == 1 {
			return chunks[3]
		}
		return nil
	}
	for pos := range SpreadBlockLight(onlyFirst, torches[:1], 1<<20) {
		if pos[0] < 0 || pos[0] > 15 || pos[2] < 16 || pos[2] > 31 {
			t.Fatalf("lit %v outside the loaded chunk", pos)
		}
	}
	at := func(x, z int) *Chunk {
		if x < 0 || z < 0 || x > 2 || z > 2 {
			return nil
		}
		return chunks[x+z*3]
	}
	if got := len(SpreadBlockLight(at, torches, 10)); got != 10 {
		t.Fatalf("lit %d positions, want the limit of 10", got)
	}
}

func BenchmarkSpreadBlockLight(b *testing.B) {
	chunks, torches := newSpreadTestArea()
	at := func(x, z int) *Chunk {
		if x < 0 || z < 0 || x > 2 || z > 2 {
			return nil
		}
		return chunks[x+z*3]
	}
	b.ReportAllocs()
	for b.Loop() {
		SpreadBlockLight(at, torches, 1<<20)
	}
}
