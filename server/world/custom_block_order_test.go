package world

import (
	"math"
	"testing"

	"github.com/df-mc/dragonfly/server/block/customblock"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// orderedBlock is a custom block with two properties and two traits.
type orderedBlock struct {
	cardinal, half string
	a              bool
	b              string
}

func (o orderedBlock) EncodeBlock() (string, map[string]any) {
	return "test:ordered", map[string]any{
		"minecraft:cardinal_direction": o.cardinal,
		"minecraft:vertical_half":      o.half,
		"test:a":                       o.a,
		"test:b":                       o.b,
	}
}
func (orderedBlock) Hash() (uint64, uint64)                  { return 0, math.MaxUint64 }
func (orderedBlock) Model() BlockModel                       { return nil }
func (orderedBlock) Properties() customblock.Properties      { return customblock.Properties{Cube: true} }
func (orderedBlock) Permutations() []customblock.Permutation { return nil }

func (orderedBlock) States() map[string][]any {
	return map[string][]any{
		"test:b": {"x", "y", "z"},
		"test:a": {false, true},
	}
}

func (orderedBlock) Traits() []customblock.Trait {
	return []customblock.Trait{
		customblock.PlacementDirection{CardinalDirection: true},
		customblock.PlacementPosition{VerticalHalf: true},
	}
}

var (
	orderedCardinal = []string{"south", "west", "north", "east"}
	orderedHalf     = []string{"bottom", "top"}
	orderedA        = []bool{false, true}
	orderedB        = []string{"x", "y", "z"}
)

// orderedState returns the state at index i of the client's enumeration: trait states first, in
// the order of the traits list and each trait's own order, then properties by name, the first
// state varying fastest.
func orderedState(i int) orderedBlock {
	c := orderedCardinal[i%4]
	i /= 4
	h := orderedHalf[i%2]
	i /= 2
	a := orderedA[i%2]
	i /= 2
	return orderedBlock{cardinal: c, half: h, a: a, b: orderedB[i]}
}

// The client gives a custom block's states runtime IDs in the order of its permutation index,
// in which the first state added to the block varies fastest. Traits add their states first
// (BlockDefinitionGroup::registerBlockFromDefinition), then the properties follow in the order
// of the definition's properties list, which dragonfly sends sorted by name.
func TestFinalizeOrdersCustomBlockStatesAsClient(t *testing.T) {
	const count = 4 * 2 * 2 * 3
	registry := NewBlockRegistry()
	// Registered in reverse, so registration order cannot pass for the client's.
	for i := count - 1; i >= 0; i-- {
		registry.RegisterBlock(orderedState(i))
	}
	registry.Finalize()

	first := registry.BlockRuntimeID(orderedState(0))
	for i := range count {
		want := orderedState(i)
		got, ok := registry.BlockByRuntimeID(first + uint32(i))
		if !ok || got != want {
			t.Fatalf("runtime ID %d = %#v, want %#v", first+uint32(i), got, want)
		}
	}
}

// A client registry built from the definitions a server sends enumerates states as the client
// does: trait states first in list order, then properties in list order, first fastest.
func TestAddCustomBlocksStateOrder(t *testing.T) {
	DefaultBlockRegistry.Finalize()
	registry := DefaultBlockRegistry.Clone()
	base := uint32(registry.BlockCount())
	entry := protocol.BlockEntry{
		Name: "test:ordered",
		Properties: map[string]any{
			"properties": []any{
				map[string]any{"name": "test:a", "enum": []any{uint8(0), uint8(1)}},
				map[string]any{"name": "test:b", "enum": []any{"x", "y", "z"}},
			},
			"traits": []any{
				map[string]any{
					"name":           "minecraft:placement_direction",
					"enabled_states": map[string]any{"cardinal_direction": uint8(1)},
				},
				map[string]any{
					"name":           "minecraft:placement_position",
					"enabled_states": map[string]any{"vertical_half": uint8(1)},
				},
			},
		},
	}
	if err := AddCustomBlocks(registry, []protocol.BlockEntry{entry}); err != nil {
		t.Fatalf("AddCustomBlocks() error = %v", err)
	}
	for i := range 4 * 2 * 2 * 3 {
		want := orderedState(i)
		a := uint8(0)
		if want.a {
			a = 1
		}
		_, got, _ := registry.RuntimeIDToState(base + uint32(i))
		if got["minecraft:cardinal_direction"] != want.cardinal || got["minecraft:vertical_half"] != want.half ||
			got["test:a"] != a || got["test:b"] != want.b {
			t.Fatalf("state %d = %v, want %+v", i, got, want)
		}
	}
}
