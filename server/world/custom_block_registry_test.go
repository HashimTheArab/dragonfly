package world

import (
	"fmt"
	"maps"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// TestAddCustomBlocksKeepsTheClientsLow16Bits lays out a block of 17 permutation bits as the client
// does: only the low 16 bits vary, so the last property keeps its first value.
func TestAddCustomBlocksKeepsTheClientsLow16Bits(t *testing.T) {
	properties := make([]any, 17)
	for i := range properties {
		properties[i] = map[string]any{
			"name": fmt.Sprintf("test:property_%d", i),
			"enum": []any{false, true},
		}
	}
	DefaultBlockRegistry.Finalize()
	registry := DefaultBlockRegistry.Clone()
	base := registry.BlockCount()
	if err := AddCustomBlocks(registry, []protocol.BlockEntry{{
		Name:       "test:block",
		Properties: map[string]any{"properties": properties},
	}}); err != nil {
		t.Fatalf("AddCustomBlocks() error = %v", err)
	}
	if got := registry.BlockCount() - base; got != 1<<16 {
		t.Fatalf("states = %d, want %d", got, 1<<16)
	}
	for _, rid := range []uint32{0, 1 << 15, 1<<16 - 1} {
		_, state, _ := registry.RuntimeIDToState(uint32(base) + rid)
		if state["test:property_16"] != false {
			t.Fatalf("state %d has test:property_16 = %v, want false", rid, state["test:property_16"])
		}
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

// Trait states take the client's value types and order.
func TestAddCustomBlocksTraitStateOrder(t *testing.T) {
	facing := []any{"down", "up", "north", "south", "west", "east"}
	for _, test := range []struct {
		trait, state string
		want         []any
	}{
		{"minecraft:placement_direction", "cardinal_direction", []any{"south", "west", "north", "east"}},
		{"minecraft:placement_direction", "facing_direction", facing},
		{"minecraft:placement_direction", "sixteen_way_rotation", []any{
			int32(0), int32(1), int32(2), int32(3), int32(4), int32(5), int32(6), int32(7),
			int32(8), int32(9), int32(10), int32(11), int32(12), int32(13), int32(14), int32(15),
		}},
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

// placement_direction states are laid out lowest bits first in the order the client adds them.
func TestAddCustomBlocksPlacementDirectionStates(t *testing.T) {
	type state struct {
		name   string
		values []any
	}
	cardinal := state{"minecraft:cardinal_direction", []any{"south", "west", "north", "east"}}
	corner := state{"minecraft:corner", []any{"none", "inner_left", "inner_right", "outer_left", "outer_right"}}
	facing := state{"minecraft:facing_direction", []any{"down", "up", "north", "south", "west", "east"}}
	rotation := state{"minecraft:sixteen_way_rotation", nil}
	for i := range int32(16) {
		rotation.values = append(rotation.values, i)
	}
	for _, test := range []struct {
		name  string
		flags map[string]any
		want  []state
	}{
		{"corner", map[string]any{"cardinal_direction": uint8(0), "corner_and_cardinal_direction": uint8(1)}, []state{cardinal, corner}},
		{"all", map[string]any{
			"cardinal_direction":            uint8(1),
			"corner_and_cardinal_direction": uint8(1),
			"facing_direction":              uint8(1),
			"sixteen_way_rotation":          uint8(1),
		}, []state{cardinal, corner, facing, rotation}},
	} {
		t.Run(test.name, func(t *testing.T) {
			DefaultBlockRegistry.Finalize()
			registry := DefaultBlockRegistry.Clone()
			base := registry.BlockCount()
			entry := protocol.BlockEntry{
				Name: "dragonfly:test_block",
				Properties: map[string]any{"traits": []any{map[string]any{
					"name":           "minecraft:placement_direction",
					"enabled_states": test.flags,
				}}},
			}
			if err := AddCustomBlocks(registry, []protocol.BlockEntry{entry}); err != nil {
				t.Fatalf("AddCustomBlocks() error = %v", err)
			}
			count := 1
			for _, s := range test.want {
				count *= len(s.values)
			}
			if got := registry.BlockCount() - base; got != count {
				t.Fatalf("states = %d, want %d", got, count)
			}
			for i := range count {
				want, index := make(map[string]any, len(test.want)), i
				for _, s := range test.want {
					want[s.name] = s.values[index%len(s.values)]
					index /= len(s.values)
				}
				if _, got, _ := registry.RuntimeIDToState(uint32(base + i)); !maps.Equal(got, want) {
					t.Fatalf("state %d = %v, want %v", i, got, want)
				}
			}
		})
	}
}
