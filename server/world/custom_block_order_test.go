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
	orderedFacing   = []string{"down", "up", "north", "south", "west", "east"}
	orderedHalf     = []string{"bottom", "top"}
	orderedA        = []bool{false, true}
	orderedB        = []string{"x", "y", "z"}
)

// orderedState returns the state at index i of the client's enumeration, the first state varying
// fastest: placement_position's states, then placement_direction's, then properties by name.
func orderedState(i int) orderedBlock {
	h := orderedHalf[i%2]
	i /= 2
	c := orderedCardinal[i%4]
	i /= 4
	a := orderedA[i%2]
	i /= 2
	return orderedBlock{cardinal: c, half: h, a: a, b: orderedB[i]}
}

// Runtime IDs follow the client's permutation index whatever the registration or trait order.
func TestFinalizeOrdersCustomBlockStatesAsClient(t *testing.T) {
	const count = 4 * 2 * 2 * 3
	registry := NewBlockRegistry()
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

// placedBlock is a custom block with every state of both placement traits, declared direction first.
type placedBlock struct{ cardinal, facing, face, half string }

func (p placedBlock) EncodeBlock() (string, map[string]any) {
	return "test:placed", map[string]any{
		"minecraft:cardinal_direction": p.cardinal,
		"minecraft:facing_direction":   p.facing,
		"minecraft:block_face":         p.face,
		"minecraft:vertical_half":      p.half,
	}
}
func (placedBlock) Hash() (uint64, uint64)             { return 0, math.MaxUint64 }
func (placedBlock) Model() BlockModel                  { return nil }
func (placedBlock) Properties() customblock.Properties { return customblock.Properties{Cube: true} }

func (placedBlock) Traits() []customblock.Trait {
	return []customblock.Trait{
		customblock.PlacementDirection{CardinalDirection: true, FacingDirection: true},
		customblock.PlacementPosition{BlockFace: true, VerticalHalf: true},
	}
}

// Offsets from the block's first runtime ID that vanilla gives these states, whichever order the
// pack declares the two traits in.
var vanillaPlacedOrder = map[uint32]placedBlock{
	0:   {"south", "down", "down", "bottom"},
	1:   {"south", "down", "up", "bottom"},
	5:   {"south", "down", "east", "bottom"},
	6:   {"south", "down", "down", "top"},
	11:  {"south", "down", "east", "top"},
	12:  {"west", "down", "down", "bottom"},
	13:  {"west", "down", "up", "bottom"},
	47:  {"east", "down", "east", "top"},
	48:  {"south", "up", "down", "bottom"},
	100: {"south", "north", "west", "bottom"},
	287: {"east", "east", "east", "top"},
}

// A block with both placement traits gets the runtime IDs vanilla gives it.
func TestFinalizeOrdersTwoTraitBlockAsVanilla(t *testing.T) {
	registry := NewBlockRegistry()
	var all []placedBlock
	for _, facing := range orderedFacing {
		for _, cardinal := range orderedCardinal {
			for _, half := range orderedHalf {
				for _, face := range orderedFacing {
					all = append(all, placedBlock{cardinal, facing, face, half})
				}
			}
		}
	}
	for i := len(all) - 1; i >= 0; i-- {
		registry.RegisterBlock(all[i])
	}
	registry.Finalize()

	first := registry.BlockRuntimeID(vanillaPlacedOrder[0])
	for i, want := range all {
		if got, _ := registry.BlockByRuntimeID(first + uint32(i)); got != want {
			t.Fatalf("offset %d = %#v, want %#v", i, got, want)
		}
	}
	for offset, want := range vanillaPlacedOrder {
		if got, _ := registry.BlockByRuntimeID(first + offset); got != want {
			t.Errorf("offset %d = %#v, want %#v", offset, got, want)
		}
	}
}

// byteBlock declares a bool property as bytes but encodes it as a bool, and a byte property the
// other way round.
type byteBlock struct {
	a bool
	b uint8
}

func (s byteBlock) EncodeBlock() (string, map[string]any) {
	return "test:bytes", map[string]any{"test:a": s.a, "test:b": s.b}
}
func (byteBlock) Hash() (uint64, uint64)                  { return 0, math.MaxUint64 }
func (byteBlock) Model() BlockModel                       { return nil }
func (byteBlock) Properties() customblock.Properties      { return customblock.Properties{Cube: true} }
func (byteBlock) Permutations() []customblock.Permutation { return nil }

func (byteBlock) States() map[string][]any {
	return map[string][]any{"test:a": {uint8(0), uint8(1)}, "test:b": {false, true}}
}

// A state value matches its declared value across bool and byte, both being a byte on the wire.
func TestFinalizeMatchesBoolAndByteStateValues(t *testing.T) {
	order := []byteBlock{{false, 0}, {true, 0}, {false, 1}, {true, 1}}
	registry := NewBlockRegistry()
	for i := len(order) - 1; i >= 0; i-- {
		registry.RegisterBlock(order[i])
	}
	registry.Finalize()

	first := registry.BlockRuntimeID(order[0])
	for i, want := range order {
		if got, _ := registry.BlockByRuntimeID(first + uint32(i)); got != want {
			t.Fatalf("runtime ID %d = %#v, want %#v", first+uint32(i), got, want)
		}
	}
}

// invalidTraitBlock enables no state of its trait, which vanilla rejects.
type invalidTraitBlock struct{ orderedBlock }

func (invalidTraitBlock) EncodeBlock() (string, map[string]any) { return "test:invalid", nil }
func (invalidTraitBlock) Traits() []customblock.Trait {
	return []customblock.Trait{customblock.PlacementDirection{}}
}

func TestRegisterBlockRejectsInvalidTraits(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("RegisterBlock did not panic")
		}
	}()
	NewBlockRegistry().RegisterBlock(invalidTraitBlock{})
}

// A client registry built from vanilla's definitions enumerates states in list order: trait states
// first, then properties, first fastest.
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
					"name":           "minecraft:placement_position",
					"enabled_states": map[string]any{"block_face": uint8(0), "vertical_half": uint8(1)},
				},
				map[string]any{
					"name":           "minecraft:placement_direction",
					"enabled_states": map[string]any{"cardinal_direction": uint8(1), "facing_direction": uint8(0)},
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
