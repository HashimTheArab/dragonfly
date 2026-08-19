package chunk

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// A sub-chunk's index width is fixed by the storage, not by how many palette entries were
// sent, so a payload can pack an index the palette does not cover. That has to resolve to
// something rather than panicking deep inside a later read.
func TestPalette_ValueToleratesAnUncoveredIndex(t *testing.T) {
	p := newPalette(4, []uint32{7, 8})
	if got := p.Value(0); got != 7 {
		t.Fatalf("Value(0) = %d, want 7", got)
	}
	if got := p.Value(15); got != 7 {
		t.Fatalf("Value(15) = %d, want the first entry (7) as the fallback", got)
	}
	if got := newPalette(4, nil).Value(3); got != 0 {
		t.Fatalf("Value on an empty palette = %d, want 0", got)
	}
}

// The count is read off the wire and used to size an allocation, so it must be bounded
// before that allocation happens rather than failing later on a short entry read.
func TestDecodePalette_RejectsHostileCounts(t *testing.T) {
	encodings := map[string]interface {
		decodePalette(buf *bytes.Buffer, blockSize paletteSize, pe paletteEncoding) (*Palette, error)
	}{
		"network":           networkEncoding{},
		"networkPersistent": networkPersistentEncoding{},
	}
	for name, e := range encodings {
		t.Run(name+"/beyond the index width", func(t *testing.T) {
			buf := &bytes.Buffer{}
			// 4 bits per block addresses 16 entries; claim more.
			_ = protocol.WriteVarint32(buf, 17)
			_, err := e.decodePalette(buf, paletteSize(4), nil)
			if err == nil {
				t.Fatal("accepted a palette larger than the storage can address")
			}
			if !strings.Contains(err.Error(), "exceeds") {
				t.Fatalf("rejected only after allocating: %v", err)
			}
		})
		t.Run(name+"/beyond the cell count", func(t *testing.T) {
			buf := &bytes.Buffer{}
			// A storage has only 4096 cells, so it cannot use more unique entries even
			// when its 16-bit indices could technically address them.
			_ = protocol.WriteVarint32(buf, 4097)
			_, err := e.decodePalette(buf, paletteSize(16), nil)
			if err == nil || !strings.Contains(err.Error(), "exceeds") {
				t.Fatalf("accepted a palette larger than the storage's cell count: %v", err)
			}
		})
		t.Run(name+"/truncated count", func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked instead of returning an error: %v", r)
				}
			}()
			if _, err := e.decodePalette(&bytes.Buffer{}, paletteSize(4), nil); err == nil {
				t.Fatal("accepted a truncated palette count")
			}
		})
		t.Run(name+"/zero count", func(t *testing.T) {
			buf := &bytes.Buffer{}
			_ = protocol.WriteVarint32(buf, 0)
			if _, err := e.decodePalette(buf, paletteSize(4), nil); err == nil {
				t.Fatal("accepted a zero palette count")
			}
		})
	}
}

func TestNetworkPersistentEncodingDecodePalette_RejectsUncountedOverflow(t *testing.T) {
	buf := &bytes.Buffer{}
	_ = protocol.WriteVarint32(buf, 2)
	enc := nbt.NewEncoderWithEncoding(buf, nbt.NetworkLittleEndian)
	for range 3 {
		if err := enc.Encode(blockEntry{Name: "test:block", State: map[string]any{}, Version: CurrentBlockVersion}); err != nil {
			t.Fatalf("encode block entry: %v", err)
		}
	}

	_, err := NetworkPersistentEncoding.decodePalette(buf, paletteSize(1), BlockPaletteEncoding{Blocks: testBlockRegistry{}})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("accepted an uncounted palette entry beyond the storage capacity: %v", err)
	}
}

func TestDecodeSubChunk_RejectsPreviousBlockStorage(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "version 1", payload: []byte{1, 0xff}},
		{name: "version 8", payload: []byte{8, 1, 0xff}},
		{name: "version 9", payload: []byte{9, 1, 0, 0xff}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := New(testBlockRegistry{}, cube.Range{0, 127})
			index := byte(0)
			sub, err := DecodeSubChunk(bytes.NewBuffer(tt.payload), ch, &index, NetworkEncoding)
			if err == nil {
				// These are the first operations Lunar performs after a successful
				// streamed-subchunk decode. Before the decoder rejects the marker,
				// Empty panics by dereferencing the nil storage it created.
				_ = sub.Empty()
				sub.Compact()
				t.Fatal("accepted a block storage that points to a previous storage")
			}
			if !strings.Contains(err.Error(), "previous") {
				t.Fatalf("error = %q, want previous-storage context", err)
			}
		})
	}
}

func TestDecodePalettedStorage_RejectsUnsupportedSize(t *testing.T) {
	for _, size := range []byte{7, 9, 15, 17, 31} {
		t.Run(fmt.Sprintf("%d bit", size), func(t *testing.T) {
			_, err := decodePalettedStorage(
				bytes.NewBuffer([]byte{size<<1 | NetworkEncoding.network()}),
				NetworkEncoding,
				BlockPaletteEncoding{Blocks: testBlockRegistry{}},
			)
			if err == nil || !strings.Contains(err.Error(), "unsupported size") {
				t.Fatalf("accepted unsupported %d-bit storage: %v", size, err)
			}
		})
	}
}

func TestDecodeSubChunk_RejectsExcessStorageLayers(t *testing.T) {
	ch := New(testBlockRegistry{}, cube.Range{0, 127})
	index := byte(0)
	_, err := DecodeSubChunk(bytes.NewBuffer([]byte{8, 3}), ch, &index, NetworkEncoding)
	if err == nil || !strings.Contains(err.Error(), "storage count") {
		t.Fatalf("accepted more storage layers than the client supports: %v", err)
	}
}

func FuzzDecodeSubChunkRuntimeOperations(f *testing.F) {
	f.Add([]byte{1, 0xff})
	f.Add([]byte{8, 1, 0xff})
	f.Add([]byte{9, 1, 0, 0xff})
	f.Add([]byte{8, 0})
	f.Add([]byte{8, 0xff})

	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > 1<<20 {
			t.Skip()
		}
		ch := New(testBlockRegistry{}, cube.Range{0, 127})
		index := byte(0)
		sub, err := DecodeSubChunk(bytes.NewBuffer(payload), ch, &index, NetworkEncoding)
		if err != nil || int(index) >= len(ch.Sub()) {
			return
		}

		if sub.Empty() {
			sub.Compact()
		}
		_ = sub.Clone()
		ch.Sub()[index] = sub
		ch.CompactForRuntimeCache()
		_ = ch.Clone()
		_ = ch.HeightMap()
		ch.SetBlock(0, 0, 0, 0, 1)
		_ = EncodeSubChunk(ch, NetworkEncoding, int(index))
	})
}

func FuzzNetworkDecodeRuntimeOperations(f *testing.F) {
	f.Add([]byte{}, byte(0))
	f.Add([]byte{1, 0xff}, byte(1))
	f.Add([]byte{9, 0, 0}, byte(1))

	f.Fuzz(func(t *testing.T, payload []byte, count byte) {
		if len(payload) > 1<<20 {
			t.Skip()
		}
		ch, _, err := NetworkDecodeWithBlockEntities(
			testBlockRegistry{}, payload, int(count), cube.Range{0, 127},
		)
		if err != nil {
			return
		}

		ch.CompactForRuntimeCache()
		_ = ch.Clone()
		_ = ch.HeightMap()
		ch.SetBlock(0, 0, 0, 0, 1)
		_ = Encode(ch, NetworkEncoding)
	})
}
