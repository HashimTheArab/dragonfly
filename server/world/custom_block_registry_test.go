package world

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestAddCustomBlocksRejectsExcessStates(t *testing.T) {
	properties := make([]any, 17)
	for i := range properties {
		properties[i] = map[string]any{
			"name": fmt.Sprintf("test:property_%d", i),
			"enum": []any{false, true},
		}
	}
	_, err := NewCustomBlockRegistry([]protocol.BlockEntry{{
		Name:       "test:block",
		Properties: map[string]any{"properties": properties},
	}})
	if err == nil || !strings.Contains(err.Error(), "exceed limit") {
		t.Fatalf("NewCustomBlockRegistry() error = %v, want state limit error", err)
	}
}

func TestAddCustomBlocksPreservesVanillaRuntimeIDs(t *testing.T) {
	DefaultBlockRegistry.Finalize()
	registry := DefaultBlockRegistry.Clone()

	oldCount := registry.BlockCount()
	oldAir := registry.AirRuntimeID()

	entry := protocol.BlockEntry{
		Name: "dragonfly:test_block",
		Properties: map[string]any{
			"properties": []any{
				map[string]any{
					"name": "dragonfly:variant",
					"enum": []any{int32(0), int32(1)},
				},
			},
		},
	}
	if err := AddCustomBlocks(registry, []protocol.BlockEntry{entry}); err != nil {
		t.Fatalf("AddCustomBlocks() error = %v", err)
	}

	if air := registry.AirRuntimeID(); air != oldAir {
		t.Fatalf("AirRuntimeID() changed: got %d, want %d", air, oldAir)
	}
	rid, ok := registry.StateToRuntimeID("dragonfly:test_block", map[string]any{"dragonfly:variant": int32(0)})
	if !ok {
		t.Fatalf("StateToRuntimeID() ok = false, want true")
	}
	if rid != uint32(oldCount) {
		t.Fatalf("StateToRuntimeID() rid = %d, want %d (custom states append after vanilla)", rid, oldCount)
	}
	if got, want := registry.BlockCount(), oldCount+2; got != want {
		t.Fatalf("BlockCount() = %d, want %d", got, want)
	}
}

func TestAddCustomBlocksAcceptsTypedEnumSlices(t *testing.T) {
	DefaultBlockRegistry.Finalize()
	registry := DefaultBlockRegistry.Clone()

	oldCount := registry.BlockCount()
	entry := protocol.BlockEntry{
		Name: "dragonfly:typed_enum_block",
		Properties: map[string]any{
			"properties": []any{
				map[string]any{
					"name": "dragonfly:toggled",
					"enum": []uint8{0, 1},
				},
				map[string]any{
					"name": "dragonfly:variant",
					"enum": []string{"a", "b", "c"},
				},
			},
		},
	}
	if err := AddCustomBlocks(registry, []protocol.BlockEntry{entry}); err != nil {
		t.Fatalf("AddCustomBlocks() error = %v", err)
	}
	if got, want := registry.BlockCount(), oldCount+6; got != want {
		t.Fatalf("BlockCount() = %d, want %d", got, want)
	}
	if _, ok := registry.StateToRuntimeID("dragonfly:typed_enum_block", map[string]any{
		"dragonfly:toggled": uint8(1), "dragonfly:variant": "b",
	}); !ok {
		t.Fatalf("StateToRuntimeID() ok = false, want true (enum values must keep their original type)")
	}
}

func TestAddCustomBlocksSkipsDuplicateStates(t *testing.T) {
	DefaultBlockRegistry.Finalize()
	registry := DefaultBlockRegistry.Clone()

	entry := protocol.BlockEntry{Name: "dragonfly:plain_block", Properties: map[string]any{}}
	if err := AddCustomBlocks(registry, []protocol.BlockEntry{entry}); err != nil {
		t.Fatalf("first AddCustomBlocks() error = %v", err)
	}
	count := registry.BlockCount()
	if err := AddCustomBlocks(registry, []protocol.BlockEntry{entry}); err != nil {
		t.Fatalf("second AddCustomBlocks() error = %v", err)
	}
	if got := registry.BlockCount(); got != count {
		t.Fatalf("BlockCount() after duplicate = %d, want %d", got, count)
	}
}

func TestNewCustomBlockRegistryPreservesVanillaRuntimeIDs(t *testing.T) {
	DefaultBlockRegistry.Finalize()

	registry, err := NewCustomBlockRegistry([]protocol.BlockEntry{{Name: "dragonfly:plain_block"}})
	if err != nil {
		t.Fatalf("NewCustomBlockRegistry() error = %v", err)
	}
	basic, ok := registry.(*BasicBlockRegistry)
	if !ok {
		t.Fatalf("NewCustomBlockRegistry() returned %T, want *BasicBlockRegistry", registry)
	}
	if got, want := basic.AirRuntimeID(), DefaultBlockRegistry.AirRuntimeID(); got != want {
		t.Fatalf("AirRuntimeID() = %d, want %d", got, want)
	}
}

// The connection trait ships enabled_states as a TAG_Int bitmask, not a map; it must
// register the four directional boolean states instead of disabling custom blocks.
func TestAddCustomBlocks_ConnectionTraitBitmask(t *testing.T) {
	registry, err := NewCustomBlockRegistry([]protocol.BlockEntry{{
		Name: "test:connectable",
		Properties: map[string]any{
			"traits": []any{map[string]any{
				"name":           "minecraft:connection",
				"enabled_states": int32(15),
			}},
		},
	}})
	if err != nil {
		t.Fatalf("NewCustomBlockRegistry() error = %v", err)
	}
	if _, ok := registry.StateToRuntimeID("test:connectable", map[string]any{
		"minecraft:connection_north": uint8(0),
		"minecraft:connection_south": uint8(1),
		"minecraft:connection_west":  uint8(0),
		"minecraft:connection_east":  uint8(1),
	}); !ok {
		t.Fatal("expected bitmask connection trait to register directional boolean states")
	}
}

// A bitmask on any other trait is unknown wire data and must fail, keeping the
// fall-back-to-vanilla-palette safety property.
func TestAddCustomBlocks_UnknownBitmaskTraitErrors(t *testing.T) {
	_, err := NewCustomBlockRegistry([]protocol.BlockEntry{{
		Name: "test:oddtrait",
		Properties: map[string]any{
			"traits": []any{map[string]any{
				"name":           "minecraft:placement_direction",
				"enabled_states": int32(3),
			}},
		},
	}})
	if err == nil {
		t.Fatal("expected an unknown bitmask trait to error")
	}
}

// Trait states take the client's values in the client's order: south, west, north, east for
// minecraft:cardinal_direction, down, up, north, south, west, east for
// minecraft:facing_direction and minecraft:block_face, all serialized as strings, the same order
// vanilla's own block states list them in (block_states.nbt).
func TestAddCustomBlocksTraitStateOrder(t *testing.T) {
	facing := []any{"down", "up", "north", "south", "west", "east"}
	for _, test := range []struct {
		trait, state string
		want         []any
	}{
		{"minecraft:placement_direction", "cardinal_direction", []any{"south", "west", "north", "east"}},
		{"minecraft:placement_direction", "facing_direction", facing},
		{"minecraft:placement_position", "block_face", facing},
		{"minecraft:placement_position", "vertical_half", []any{"bottom", "top"}},
	} {
		t.Run(test.state, func(t *testing.T) {
			DefaultBlockRegistry.Finalize()
			registry := DefaultBlockRegistry.Clone()
			base := uint32(registry.BlockCount())
			entry := protocol.BlockEntry{
				Name: "dragonfly:test_block",
				Properties: map[string]any{"traits": []any{map[string]any{
					"name":           test.trait,
					"enabled_states": map[string]any{test.state: uint8(1)},
				}}},
			}
			if err := AddCustomBlocks(registry, []protocol.BlockEntry{entry}); err != nil {
				t.Fatalf("AddCustomBlocks() error = %v", err)
			}
			if got := registry.BlockCount() - int(base); got != len(test.want) {
				t.Fatalf("states = %d, want %d", got, len(test.want))
			}
			for i, want := range test.want {
				_, properties, _ := registry.RuntimeIDToState(base + uint32(i))
				if got := properties["minecraft:"+test.state]; got != want {
					t.Errorf("state %d = %v, want %v", i, got, want)
				}
			}
		})
	}
}
