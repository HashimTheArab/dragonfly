package world

import (
	"reflect"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// testVanillaBlockData returns the vanilla_block_data compound every block definition a client
// accepts carries.
func testVanillaBlockData() map[string]any {
	return map[string]any{"block_id": int32(10000), "material": "dirt"}
}

// testDefinition returns a StartGame block definition with fields merged over vanilla_block_data.
func testDefinition(name string, fields map[string]any) protocol.BlockEntry {
	properties := map[string]any{"vanilla_block_data": testVanillaBlockData()}
	for k, v := range fields {
		properties[k] = v
	}
	return protocol.BlockEntry{Name: name, Properties: properties}
}

// testDigestStates returns the states the client registers for one definition, in runtime ID order.
func testDigestStates(t *testing.T, entry protocol.BlockEntry) []map[string]any {
	t.Helper()
	DefaultBlockRegistry.Finalize()
	registry := DefaultBlockRegistry.Clone()
	base := registry.BlockCount()
	if err := AddCustomBlocks(registry, []protocol.BlockEntry{entry}); err != nil {
		t.Fatalf("AddCustomBlocks() error = %v", err)
	}
	var states []map[string]any
	for rid := base; rid < registry.BlockCount(); rid++ {
		_, properties, _ := registry.RuntimeIDToState(uint32(rid))
		states = append(states, properties)
	}
	return states
}

func TestDigestNetworkBlocks_EntryRules(t *testing.T) {
	variant := func(values ...any) map[string]any {
		return map[string]any{"properties": []any{map[string]any{"name": "test:v", "enum": values}}}
	}
	definitions, err := digestNetworkBlocks([]protocol.BlockEntry{
		{Name: "test:no_vanilla_data", Properties: variant(int32(0))},
		{Name: "test:wrong_vanilla_data", Properties: map[string]any{"vanilla_block_data": int32(1)}},
		testDefinition("test:repeated", variant(int32(0))),
		testDefinition("test:kept", nil),
		testDefinition("test:repeated", variant(int32(0), int32(1))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 2 || definitions[0].name != "test:repeated" || definitions[1].name != "test:kept" {
		t.Fatalf("definitions = %+v, want test:repeated then test:kept", definitions)
	}
	if got := len(definitions[0].states[0].Values); got != 2 {
		t.Fatalf("repeated definition has %d values, want the later entry's 2", got)
	}
}

func TestDigestNetworkBlocks_MalformedPropertiesSkipOneElement(t *testing.T) {
	states := testDigestStates(t, testDefinition("test:block", map[string]any{"properties": []any{
		"not a compound",
		map[string]any{"name": "test:no_enum"},
		map[string]any{"name": "test:scalar_enum", "enum": int32(3)},
		map[string]any{"enum": []string{"x", "y"}},
		map[string]any{"name": "test:kept", "enum": []int16{4, 5}},
	}}))
	want := []map[string]any{
		{"": "x", "test:kept": int16(4)},
		{"": "y", "test:kept": int16(4)},
		{"": "x", "test:kept": int16(5)},
		{"": "y", "test:kept": int16(5)},
	}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("states = %v, want %v", states, want)
	}
}

func TestDigestNetworkBlocks_EmptyEnumIsRejected(t *testing.T) {
	_, err := NewCustomBlockRegistry([]protocol.BlockEntry{testDefinition("test:block", map[string]any{
		"properties": []any{map[string]any{"name": "test:empty", "enum": []any{}}},
	})})
	if err == nil {
		t.Fatal("palette with an empty enum loaded")
	}
}

func TestDigestNetworkBlocks_TraitRules(t *testing.T) {
	for _, test := range []struct {
		name   string
		traits []any
		want   []string
	}{
		{"unknown and nameless traits add nothing", []any{
			map[string]any{"name": "test:unknown", "enabled_states": map[string]any{"block_face": uint8(1)}},
			map[string]any{"enabled_states": map[string]any{"block_face": uint8(1)}},
			"not a compound",
		}, nil},
		{"names are case-insensitive", []any{
			map[string]any{"name": "Minecraft:Placement_Position", "enabled_states": map[string]any{"vertical_half": uint8(1)}},
		}, []string{"minecraft:vertical_half"}},
		{"the first trait with a name wins", []any{
			map[string]any{"name": "minecraft:placement_position", "enabled_states": map[string]any{"vertical_half": uint8(1)}},
			map[string]any{"name": "minecraft:placement_position", "enabled_states": map[string]any{"block_face": uint8(1)}},
		}, []string{"minecraft:vertical_half"}},
		{"only byte flags enable states", []any{
			map[string]any{"name": "minecraft:placement_position", "enabled_states": map[string]any{"block_face": int32(1), "vertical_half": uint8(1)}},
		}, []string{"minecraft:vertical_half"}},
		{"placement direction adds states in the client's order", []any{
			map[string]any{"name": "minecraft:placement_direction", "enabled_states": map[string]any{
				"sixteen_way_rotation": uint8(1), "facing_direction": uint8(1), "corner_and_cardinal_direction": uint8(1),
			}},
		}, []string{"minecraft:cardinal_direction", "minecraft:corner", "minecraft:facing_direction", "minecraft:sixteen_way_rotation"}},
		{"traits apply in list order", []any{
			map[string]any{"name": "minecraft:placement_direction", "enabled_states": map[string]any{"facing_direction": uint8(1)}},
			map[string]any{"name": "minecraft:placement_position", "enabled_states": map[string]any{"block_face": uint8(1)}},
		}, []string{"minecraft:facing_direction", "minecraft:block_face"}},
		{"connection tests only the lowest bit", []any{
			map[string]any{"name": "minecraft:connection", "enabled_states": int32(2)},
		}, nil},
		{"connection adds north, east, south and west", []any{
			map[string]any{"name": "minecraft:connection", "enabled_states": int32(1)},
		}, []string{"minecraft:connection_north", "minecraft:connection_east", "minecraft:connection_south", "minecraft:connection_west"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			states, err := networkBlockStates(map[string]any{"traits": test.traits})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, state := range states {
				got = append(got, state.Name)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("states = %v, want %v", got, test.want)
			}
		})
	}
}

// Trait states come before properties, and the first state added varies fastest.
func TestDigestNetworkBlocks_TraitStatesPrecedeProperties(t *testing.T) {
	states := testDigestStates(t, testDefinition("test:block", map[string]any{
		"properties": []any{map[string]any{"name": "test:lit", "enum": []uint8{0, 1}}},
		"traits": []any{map[string]any{
			"name": "minecraft:placement_position", "enabled_states": map[string]any{"vertical_half": uint8(1)},
		}},
	}))
	want := []map[string]any{
		{"minecraft:vertical_half": "bottom", "test:lit": uint8(0)},
		{"minecraft:vertical_half": "top", "test:lit": uint8(0)},
		{"minecraft:vertical_half": "bottom", "test:lit": uint8(1)},
		{"minecraft:vertical_half": "top", "test:lit": uint8(1)},
	}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("states = %v, want %v", states, want)
	}
}

func TestNewNetworkBlockRegistry_OptionalBlockNeedsVanillaBlockData(t *testing.T) {
	block := DataDrivenBlocks()[0]
	registry, err := NewNetworkBlockRegistry([]protocol.BlockEntry{{Name: block.Name, Properties: map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.StateToRuntimeID(block.Name, nil); ok {
		t.Fatalf("%s participates without vanilla_block_data", block.Name)
	}
}

func TestNetworkBlockHash_LowercasesName(t *testing.T) {
	if NetworkBlockHash("Test:Block") != NetworkBlockHash("test:block") {
		t.Fatal("palette hash depends on the case of a name")
	}
}
