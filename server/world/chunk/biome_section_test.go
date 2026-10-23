package chunk

import (
	"bytes"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
)

// TestSetBiomeSection checks replacement, compact storage and every position across negative heights.
func TestSetBiomeSection(t *testing.T) {
	c := New(testBlockRegistry{}, cube.Range{-64, 319})
	for section := range c.biomes {
		y := int16(-64 + section*16)
		c.SetBiome(3, y, 5, 100)
		// An interior height must select the same section as its minimum height.
		c.SetBiomeSection(y+7, uint32(section%3))
		storage := c.biomes[section]
		if storage.bitsPerIndex != 0 || len(storage.indices) != 0 {
			t.Fatalf("section %d: uniform storage retains indices", section)
		}
		if storage.palette.Len() != 1 || storage.palette.Value(0) != uint32(section%3) {
			t.Fatalf("section %d: palette = %v", section, storage.palette.values)
		}
	}
	for y := int16(-64); y < 320; y++ {
		for z := uint8(0); z < 16; z++ {
			for x := uint8(0); x < 16; x++ {
				want := uint32(c.SubIndex(y) % 3)
				if got := c.Biome(x, y, z); got != want {
					t.Fatalf("(%d,%d,%d): got %d, want %d", x, y, z, got, want)
				}
			}
		}
	}
}

// TestSetBiomeSectionMutation checks that scalar writes preserve neighbouring cells, sections and chunks.
func TestSetBiomeSectionMutation(t *testing.T) {
	c := New(testBlockRegistry{}, cube.Range{-64, -33})
	other := New(testBlockRegistry{}, c.Range())
	for _, target := range []*Chunk{c, other} {
		target.SetBiomeSection(-64, 7)
		target.SetBiomeSection(-48, 7)
	}
	c.SetBiome(3, -60, 5, 9)
	for _, target := range []*Chunk{c, other} {
		for y := int16(-64); y < -32; y++ {
			for z := uint8(0); z < 16; z++ {
				for x := uint8(0); x < 16; x++ {
					want := uint32(7)
					if target == c && x == 3 && y == -60 && z == 5 {
						want = 9
					}
					if got := target.Biome(x, y, z); got != want {
						t.Fatalf("(%d,%d,%d): got %d, want %d", x, y, z, got, want)
					}
				}
			}
		}
	}
}

// TestSetBiomeSectionEncoding checks compact biomes survive both supported palette encodings.
func TestSetBiomeSectionEncoding(t *testing.T) {
	for _, encoding := range []Encoding{NetworkEncoding, DiskEncoding} {
		c := New(testBlockRegistry{}, cube.Range{-64, -33})
		c.SetBiomeSection(-64, 0)
		c.SetBiomeSection(-48, 7)
		buf := bytes.NewBuffer(EncodeBiomes(c, encoding))
		for _, want := range []uint32{0, 7} {
			storage, err := decodePalettedStorage(buf, encoding, BiomePaletteEncoding)
			if err != nil {
				t.Fatal(err)
			}
			if storage == nil || storage.bitsPerIndex != 0 || storage.palette.Len() != 1 || storage.At(15, 15, 15) != want {
				t.Fatalf("invalid decoded uniform biome %d", want)
			}
		}
		if buf.Len() != 0 {
			t.Fatalf("%d trailing biome bytes", buf.Len())
		}
	}
}
