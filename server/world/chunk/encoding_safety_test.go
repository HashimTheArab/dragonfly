package chunk

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestNetworkPersistentEncodingDecodePalette_TruncatedCountReturnsError(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x80})
	_, err := NetworkPersistentEncoding.decodePalette(buf, 1, BlockPaletteEncoding{Blocks: testBlockRegistry{}})
	if err == nil {
		t.Fatal("expected truncated palette count to return an error")
	}
}

func TestNetworkEncodingDecodePalette_RejectsCountBeyondBitCapacity(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	_ = protocol.WriteVarint32(buf, 3)
	for range 3 {
		_ = protocol.WriteVarint32(buf, 0)
	}

	_, err := NetworkEncoding.decodePalette(buf, 1, BlockPaletteEncoding{Blocks: testBlockRegistry{}})
	if err == nil || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("expected capacity error, got %v", err)
	}
}

func TestNetworkPersistentEncodingDecodePalette_RejectsDuplicateOverflow(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	_ = protocol.WriteVarint32(buf, 2)
	enc := nbt.NewEncoderWithEncoding(buf, nbt.NetworkLittleEndian)
	for range 3 {
		if err := enc.Encode(blockEntry{Name: "test:block", State: map[string]any{}, Version: CurrentBlockVersion}); err != nil {
			t.Fatalf("encode block entry: %v", err)
		}
	}

	_, err := NetworkPersistentEncoding.decodePalette(buf, 1, BlockPaletteEncoding{Blocks: testBlockRegistry{}})
	if err == nil || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("expected duplicate overflow capacity error, got %v", err)
	}
}

func TestDecodePalettedStorage_RejectsMissingPaletteEntry(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte(3) // One bit per block using network runtime IDs.
	if err := binary.Write(buf, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("write first packed index: %v", err)
	}
	buf.Write(make([]byte, paletteSize(1).uint32s()*4-4))
	_ = protocol.WriteVarint32(buf, 1)
	_ = protocol.WriteVarint32(buf, 0)

	_, err := decodePalettedStorage(buf, NetworkEncoding, BlockPaletteEncoding{Blocks: testBlockRegistry{}})
	if err == nil || !strings.Contains(err.Error(), "palette index") {
		t.Fatalf("expected missing palette entry error, got %v", err)
	}
}
