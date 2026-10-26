package world

import (
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestNewNetworkBlockRegistry_NegotiatedVanillaBlocks(t *testing.T) {
	DefaultBlockRegistry.Finalize()
	all := make([]protocol.BlockEntry, 0, len(DataDrivenBlocks()))
	for _, block := range DataDrivenBlocks() {
		all = append(all, protocol.BlockEntry{Name: block.Name, Properties: block.Components})
	}
	for _, tc := range []struct {
		name    string
		entries []protocol.BlockEntry
	}{
		{"none", nil},
		{"one", all[:1]},
		{"all", all},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry, err := NewNetworkBlockRegistry(tc.entries)
			if err != nil {
				t.Fatal(err)
			}
			advertised := map[string]bool{}
			for _, entry := range tc.entries {
				advertised[entry.Name] = true
			}
			for _, block := range DataDrivenBlocks() {
				if _, ok := registry.StateToRuntimeID(block.Name, nil); ok != advertised[block.Name] {
					t.Errorf("%s present = %t, advertised = %t", block.Name, ok, advertised[block.Name])
				}
			}
			if tc.name == "all" {
				for rid, block := range DefaultBlockRegistry.Blocks() {
					name, properties := block.EncodeBlock()
					if got, ok := registry.StateToRuntimeID(name, properties); !ok || got != uint32(rid) {
						t.Fatalf("%s runtime ID = (%d, %t), want %d", name, got, ok, rid)
					}
				}
			}
		})
	}
}

func TestNewNetworkBlockRegistry_BuiltInRuntimeIDsWithoutOptionalBlocks(t *testing.T) {
	registry, err := NewNetworkBlockRegistry([]protocol.BlockEntry{{Name: "lunar:abgbzvnw0"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		id   uint32
	}{
		{"minecraft:oak_log", 1807}, {"minecraft:stone", 3219},
		{"minecraft:grass_block", 13859}, {"minecraft:barrier", 14409}, {"minecraft:air", 15844},
	} {
		if got, ok := registry.StateToRuntimeID(tc.name, nil); !ok || got != tc.id {
			t.Errorf("%s runtime ID = (%d, %t), want %d", tc.name, got, ok, tc.id)
		}
	}
	custom, ok := registry.StateToRuntimeID("lunar:abgbzvnw0", nil)
	if !ok {
		t.Fatal("custom state missing")
	}
	hash, ok := registry.RuntimeIDToHash(custom)
	if !ok {
		t.Fatal("custom state hash missing")
	}
	if back, ok := registry.HashToRuntimeID(hash); !ok || back != custom {
		t.Fatal("custom hash did not round-trip")
	}
}
