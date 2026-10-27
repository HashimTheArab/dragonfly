package world

import (
	"fmt"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// BenchmarkNetworkBlockPalette measures definition parsing and complete registry construction.
func BenchmarkNetworkBlockPalette(b *testing.B) {
	DefaultBlockRegistry.Finalize()
	vanilla := make([]protocol.BlockEntry, 0, len(DataDrivenBlocks()))
	for _, block := range DataDrivenBlocks() {
		vanilla = append(vanilla, protocol.BlockEntry{Name: block.Name, Properties: block.Components})
	}
	for _, test := range []struct {
		name    string
		entries []protocol.BlockEntry
	}{
		{"Empty", nil},
		{"Vanilla", vanilla},
		{"Custom128", benchmarkNetworkBlockEntries(128, false)},
		{"Repeated128", benchmarkNetworkBlockEntries(128, true)},
	} {
		b.Run(test.name, func(b *testing.B) {
			b.Run("Digest", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := digestNetworkBlocks(test.entries); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("Registry", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := NewNetworkBlockRegistry(test.entries); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// benchmarkNetworkBlockEntries builds eight-state definitions, optionally replacing every other entry.
func benchmarkNetworkBlockEntries(count int, repeated bool) []protocol.BlockEntry {
	entries := make([]protocol.BlockEntry, count)
	for i := range entries {
		name := i
		if repeated {
			name /= 2
		}
		entries[i] = testDefinition(fmt.Sprintf("bench:block_%d", name), map[string]any{
			"traits": []any{map[string]any{
				"name":           "minecraft:placement_position",
				"enabled_states": map[string]any{"vertical_half": uint8(1)},
			}},
			"properties": []any{
				map[string]any{"name": "bench:colour", "enum": []string{"red", "blue"}},
				map[string]any{"name": "bench:level", "enum": []int32{0, 1}},
			},
		})
	}
	return entries
}
