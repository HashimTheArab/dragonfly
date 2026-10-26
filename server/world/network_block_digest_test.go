package world

import (
	"reflect"
	"testing"

	"github.com/df-mc/dragonfly/server/block/customblock"
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
		{"connection's second bit adds up and down", []any{
			map[string]any{"name": "minecraft:connection", "enabled_states": int32(2)},
		}, []string{"minecraft:connection_up", "minecraft:connection_down"}},
		{"connection adds north, east, south and west", []any{
			map[string]any{"name": "minecraft:connection", "enabled_states": int32(1)},
		}, []string{"minecraft:connection_north", "minecraft:connection_east", "minecraft:connection_south", "minecraft:connection_west"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			states, _, err := networkBlockStates(map[string]any{"traits": test.traits})
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

func TestDigestNetworkBlocks_AnyTagTypeStateValues(t *testing.T) {
	for name, values := range map[string]any{
		"long":       []int64{1, 2},
		"float":      []float32{1, 2},
		"double":     []float64{1, 2},
		"list":       []any{[]any{int32(1)}, []any{"1"}},
		"compound":   []any{map[string]any{"a": int32(1)}, map[string]any{"a": int16(1)}},
		"byte array": []any{[2]uint8{1, 2}, [2]uint8{2, 1}},
		"int array":  []any{[1]int32{1}, [1]int32{2}},
	} {
		t.Run(name, func(t *testing.T) {
			registry, err := NewNetworkBlockRegistry([]protocol.BlockEntry{testDefinition("test:block", map[string]any{
				"properties": []any{map[string]any{"name": "test:v", "enum": values}},
			})})
			if err != nil {
				t.Fatal(err)
			}
			hashes := make(map[uint32]struct{})
			for _, value := range []any{reflect.ValueOf(values).Index(0).Interface(), reflect.ValueOf(values).Index(1).Interface()} {
				rid, ok := registry.StateToRuntimeID("test:block", map[string]any{"test:v": value})
				if !ok {
					t.Fatalf("state %v has no runtime ID", value)
				}
				if b, ok := registry.BlockByName("test:block", map[string]any{"test:v": value}); !ok || !IsUnknownBlock(b) {
					t.Fatalf("BlockByName(%v) = %v, %v", value, b, ok)
				}
				hash, _ := registry.RuntimeIDToHash(rid)
				hashes[hash] = struct{}{}
			}
			if len(hashes) != 2 {
				t.Fatal("distinct states share a network hash")
			}
		})
	}
}

// The client gives every permutation its own runtime ID, even when two serialize the same.
func TestDigestNetworkBlocks_RepeatedStatesKeepRuntimeIDs(t *testing.T) {
	for name, properties := range map[string][]any{
		"repeated enum value": {map[string]any{"name": "test:v", "enum": []string{"a", "a"}}},
		"repeated property":   {map[string]any{"name": "test:v", "enum": []uint8{0, 1}}, map[string]any{"name": "test:v", "enum": []uint8{0, 1}}},
		"concatenated values": {map[string]any{"name": "test:a", "enum": []string{"x", "xy"}}, map[string]any{"name": "test:b", "enum": []string{"yz", "z"}}},
	} {
		t.Run(name, func(t *testing.T) {
			states := 1
			for _, property := range properties {
				states *= reflect.ValueOf(property.(map[string]any)["enum"]).Len()
			}
			registry, err := NewNetworkBlockRegistry([]protocol.BlockEntry{testDefinition("test:block", map[string]any{"properties": properties})})
			if err != nil {
				t.Fatal(err)
			}
			baseline, _ := NewNetworkBlockRegistry(nil)
			if got := registry.BlockCount() - baseline.BlockCount(); got != states {
				t.Fatalf("states = %d, want %d", got, states)
			}
			// States of one block that serialize the same share a network hash; all others differ.
			hashes, distinct := make(map[uint32]struct{}), make(map[string]struct{})
			for rid := range registry.BlockCount() {
				hash, _ := registry.RuntimeIDToHash(uint32(rid))
				hashes[hash] = struct{}{}
				name, properties, _ := registry.RuntimeIDToState(uint32(rid))
				distinct[name+hashProperties(properties)] = struct{}{}
			}
			if len(hashes) != len(distinct) {
				t.Fatalf("%d network hashes for %d distinct states", len(hashes), len(distinct))
			}
		})
	}
}

func TestDigestNetworkBlocks_CaseVariantsRegisterOnce(t *testing.T) {
	variant := func(n int) map[string]any {
		values := make([]int32, n)
		return map[string]any{"properties": []any{map[string]any{"name": "test:v", "enum": values}}}
	}
	registry, err := NewNetworkBlockRegistry([]protocol.BlockEntry{
		testDefinition("Test:Block", variant(2)),
		testDefinition("test:block", variant(3)),
	})
	if err != nil {
		t.Fatal(err)
	}
	baseline, _ := NewNetworkBlockRegistry(nil)
	if got := registry.BlockCount() - baseline.BlockCount(); got != 2 {
		t.Fatalf("states = %d, want the first name's 2", got)
	}
}

func TestNetworkBlockHash_ClientNameRules(t *testing.T) {
	if NetworkBlockHash("test:block\x00ignored") != NetworkBlockHash("test:block") {
		t.Fatal("hash continues past a NUL byte")
	}
	if NetworkBlockHash("test:É") == NetworkBlockHash("test:é") {
		t.Fatal("non-ASCII letters were lowercased")
	}
}

func TestDigestNetworkBlocks_ZeroPartMultiBlock(t *testing.T) {
	states := testDigestStates(t, testDefinition("test:block", map[string]any{"traits": []any{map[string]any{
		"name": "minecraft:multi_block", "enabled_states": map[string]any{"multi_block": uint8(1)}, "part_count": uint8(0),
	}}}))
	if want := []map[string]any{{"minecraft:multi_block_part": int32(0)}}; !reflect.DeepEqual(states, want) {
		t.Fatalf("states = %v, want %v", states, want)
	}
}

func TestAddCustomBlocks_StateLimitLeavesRegistryUnchanged(t *testing.T) {
	properties := make([]any, 16)
	for i := range properties {
		properties[i] = map[string]any{"name": "test:p" + string(rune('a'+i)), "enum": []uint8{0, 1}}
	}
	DefaultBlockRegistry.Finalize()
	registry := DefaultBlockRegistry.Clone()
	before := registry.BlockCount()
	err := AddCustomBlocks(registry, []protocol.BlockEntry{
		testDefinition("test:first", map[string]any{"properties": properties}),
		testDefinition("test:second", map[string]any{"properties": properties}),
	})
	if err == nil {
		t.Fatal("palette past the state limit loaded")
	}
	if registry.BlockCount() != before {
		t.Fatal("registry changed before the state limit error")
	}
}

func TestNetworkStatesEqual_TagTypesAndBits(t *testing.T) {
	bools := []customblock.TraitState{{Name: "test:v", Values: []any{false, true}}}
	bytesValues := []customblock.TraitState{{Name: "test:v", Values: []any{uint8(0), uint8(1)}}}
	ints := []customblock.TraitState{{Name: "test:v", Values: []any{int32(0), int32(1)}}}
	if !networkStatesEqual(bools, bytesValues) {
		t.Fatal("bool and byte values differ")
	}
	if networkStatesEqual(bools, ints) {
		t.Fatal("bool and int values match")
	}
	wide := make([]customblock.TraitState, 17)
	for i := range wide {
		wide[i] = customblock.TraitState{Name: "test:p" + string(rune('a'+i)), Values: []any{false, true}}
	}
	if networkStatesEqual(wide, wide) {
		t.Fatal("states past 16 bits match")
	}
}

func TestDigestNetworkBlocks_LongArrayIsRejected(t *testing.T) {
	_, err := NewNetworkBlockRegistry([]protocol.BlockEntry{testDefinition("test:block", map[string]any{
		"properties": []any{map[string]any{"name": "test:v", "enum": []any{[1]int64{1}, [1]int64{2}}}},
	})})
	if err == nil {
		t.Fatal("palette with a long array loaded")
	}
}

// A field crossing bit 16 never invalidates a permutation, but its value comes from the data bits
// it has: data value 1<<15 sets the second value.
func TestDigestNetworkBlocks_CrossingFieldTakesItsDataBits(t *testing.T) {
	properties := make([]any, 0, 16)
	for i := range 15 {
		properties = append(properties, map[string]any{"name": "test:p" + string(rune('a'+i)), "enum": []uint8{0, 1}})
	}
	properties = append(properties, map[string]any{"name": "test:q", "enum": []uint8{0, 1, 2, 3}})
	states := testDigestStates(t, testDefinition("test:block", map[string]any{"properties": properties}))
	if len(states) != 1<<16 {
		t.Fatalf("states = %d, want %d", len(states), 1<<16)
	}
	if states[0]["test:q"] != uint8(0) || states[1<<15]["test:q"] != uint8(1) {
		t.Fatalf("test:q = %v and %v, want 0 and 1", states[0]["test:q"], states[1<<15]["test:q"])
	}
}

func TestDigestNetworkBlocks_MultiBlockPartsPastFourAreZero(t *testing.T) {
	states := testDigestStates(t, testDefinition("test:block", map[string]any{"traits": []any{map[string]any{
		"name": "minecraft:multi_block", "enabled_states": map[string]any{"multi_block": uint8(1)}, "part_count": uint8(5),
	}}}))
	var got []any
	for _, state := range states {
		got = append(got, state["minecraft:multi_block_part"])
	}
	if want := []any{int32(0), int32(1), int32(2), int32(3), int32(0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parts = %v, want %v", got, want)
	}
}

// States of one block sharing a network hash resolve to the last of them.
func TestNetworkHash_SharedHashResolvesToLastState(t *testing.T) {
	registry, err := NewNetworkBlockRegistry([]protocol.BlockEntry{testDefinition("test:dup", map[string]any{
		"properties": []any{map[string]any{"name": "test:p", "enum": []uint8{0, 0}}},
	})})
	if err != nil {
		t.Fatal(err)
	}
	first, ok := registry.StateToRuntimeID("test:dup", map[string]any{"test:p": uint8(0)})
	if !ok {
		t.Fatal("state missing")
	}
	hash, _ := registry.RuntimeIDToHash(first)
	if second, _ := registry.RuntimeIDToHash(first + 1); second != hash {
		t.Fatal("repeated states of one block have different hashes")
	}
	if rid, _ := registry.HashToRuntimeID(hash); rid != first+1 {
		t.Fatalf("hash resolves to %d, want the last state %d", rid, first+1)
	}
}

// Probing for a free network hash wraps to zero past the reserved values.
func TestNetworkHash_ProbeWrapsPastReservedValues(t *testing.T) {
	registry := &BasicBlockRegistry{networkhashToRids: map[uint32]uint32{0: 0, 0xfffffffd: 1}}
	registry.blocks = []Block{unknownBlock{BlockState: BlockState{Name: "test:a"}}, unknownBlock{BlockState: BlockState{Name: "test:b"}}}
	if got := registry.networkHashLocked(0xfffffffd, "test:c"); got != 1 {
		t.Fatalf("hash = %#x, want 1", got)
	}
}

// A held hash moves up from where it is; only a held hash past the reserved values wraps to zero.
func TestNetworkHash_ProbeFromHeldHashBelowReservedValues(t *testing.T) {
	registry := &BasicBlockRegistry{networkhashToRids: map[uint32]uint32{0xfffffffc: 0}}
	registry.blocks = []Block{unknownBlock{BlockState: BlockState{Name: "test:a"}}}
	if got := registry.networkHashLocked(0xfffffffc, "test:b"); got != 0xfffffffd {
		t.Fatalf("hash = %#x, want 0xfffffffd", got)
	}
	if got := registry.networkHashLocked(0xfffffffe, "test:b"); got != 0xfffffffe {
		t.Fatalf("free reserved hash = %#x, want it taken as is", got)
	}
}

// A 17-bit field reads no bits, so all of its 65536 permutations take its first value.
func TestDigestNetworkBlocks_WideFieldReadsNoBits(t *testing.T) {
	values := make([]int32, 1<<16+1)
	for i := range values {
		values[i] = int32(i)
	}
	states := testDigestStates(t, testDefinition("test:block", map[string]any{
		"properties": []any{map[string]any{"name": "test:p", "enum": values}},
	}))
	if len(states) != 1<<16 {
		t.Fatalf("states = %d, want %d", len(states), 1<<16)
	}
	for i, state := range states {
		if state["test:p"] != int32(0) {
			t.Fatalf("state %d sets test:p to %v", i, state["test:p"])
		}
	}
}

// A field starting at bit 32 wraps its shift and reads the lowest data bit.
func TestDigestNetworkBlocks_FieldAtBit32ReusesLowBits(t *testing.T) {
	values := make([]int32, 256)
	for i := range values {
		values[i] = int32(i)
	}
	var properties []any
	for i := range 4 {
		properties = append(properties, map[string]any{"name": "test:p" + string(rune('a'+i)), "enum": values})
	}
	properties = append(properties, map[string]any{"name": "test:last", "enum": []uint8{0, 1}})
	states := testDigestStates(t, testDefinition("test:block", map[string]any{"properties": properties}))
	if len(states) != 1<<16 {
		t.Fatalf("states = %d, want %d", len(states), 1<<16)
	}
	if states[0]["test:last"] != uint8(0) || states[1]["test:last"] != uint8(1) {
		t.Fatalf("test:last = %v and %v, want 0 and 1", states[0]["test:last"], states[1]["test:last"])
	}
	if states[1<<8]["test:pc"] != int32(0) {
		t.Fatalf("field at bit 16 reads %v, want 0", states[1<<8]["test:pc"])
	}
}

func TestNewNetworkBlockRegistry_UnqualifiedNameIsVanillaNamespace(t *testing.T) {
	registry, err := NewNetworkBlockRegistry([]protocol.BlockEntry{testDefinition("parity_unqualified", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.StateToRuntimeID("minecraft:parity_unqualified", map[string]any{}); !ok {
		t.Fatal("unqualified block is not in the minecraft namespace")
	}
}

func TestNetworkBlockHash_EmptyNameIsZero(t *testing.T) {
	if NetworkBlockHash("") != 0 || NetworkBlockHash("\x00:test") != 0 {
		t.Fatal("a name starting with NUL does not hash to zero")
	}
}

// The unknown block keeps its reserved hash, and the hash resolves to it, even when a block before
// it already took the same value.
func TestNetworkHash_UnknownBlockKeepsReservedHash(t *testing.T) {
	registry, err := NewNetworkBlockRegistry([]protocol.BlockEntry{testDefinition("test:reserved0", map[string]any{
		"properties": []any{map[string]any{"name": "test:p", "enum": []int32{1002028478}}},
	})})
	if err != nil {
		t.Fatal(err)
	}
	unknown, ok := registry.StateToRuntimeID("minecraft:unknown", nil)
	if !ok {
		t.Skip("registry has no minecraft:unknown")
	}
	if hash, _ := registry.RuntimeIDToHash(unknown); hash != 0xfffffffe {
		t.Fatalf("unknown block hash = %#x, want 0xfffffffe", hash)
	}
	if rid, _ := registry.HashToRuntimeID(0xfffffffe); rid != unknown {
		t.Fatalf("0xfffffffe resolves to %d, want the unknown block %d", rid, unknown)
	}
}

// A later definition naming a block in the minecraft namespace adds its states to the block an
// earlier definition registered under the same name, past the block's own properties; without the
// namespace it is ignored.
func TestDigestNetworkBlocks_NamespaceAliasAddsStates(t *testing.T) {
	property := func(name string) map[string]any {
		return map[string]any{"properties": []any{map[string]any{"name": name, "enum": []uint8{0, 1}}}}
	}
	for _, test := range []struct {
		name    string
		entries []protocol.BlockEntry
		want    int
	}{
		{"qualified alias adds states past the block's own", []protocol.BlockEntry{
			testDefinition("parity_alias", property("test:p")),
			testDefinition("minecraft:parity_alias", map[string]any{"properties": []any{
				map[string]any{"name": "test:q", "enum": []uint8{0, 1}},
				map[string]any{"name": "test:r", "enum": []uint8{0, 1}},
			}}),
		}, 4},
		{"unqualified alias is ignored", []protocol.BlockEntry{
			testDefinition("minecraft:parity_alias", property("test:p")),
			testDefinition("parity_alias", property("test:q")),
		}, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			registry, err := NewNetworkBlockRegistry(test.entries)
			if err != nil {
				t.Fatal(err)
			}
			baseline, _ := NewNetworkBlockRegistry(nil)
			if got := registry.BlockCount() - baseline.BlockCount(); got != test.want {
				t.Fatalf("states = %d, want %d", got, test.want)
			}
		})
	}
}

// An extending definition numbers its properties from the same first state ID as the block's own,
// so a property at a position the block already has is ignored, and only later ones are added.
// Its trait states are matched by trait, never against a property of the same name.
func TestDigestNetworkBlocks_ExtendingDefinitionStateIDs(t *testing.T) {
	property := func(name string) map[string]any {
		return map[string]any{"name": name, "enum": []uint8{0, 1}}
	}
	for _, test := range []struct {
		name    string
		entries []protocol.BlockEntry
		want    int
	}{
		{"property at a taken position", []protocol.BlockEntry{
			testDefinition("minecraft:parity_r5", map[string]any{"properties": []any{property("test:p")}}),
			testDefinition("Minecraft:parity_r5", map[string]any{"properties": []any{property("test:q")}}),
		}, 2},
		{"property past the block's count", []protocol.BlockEntry{
			testDefinition("minecraft:parity_r5", map[string]any{"properties": []any{property("test:p")}}),
			testDefinition("Minecraft:parity_r5", map[string]any{"properties": []any{property("test:q"), property("test:r")}}),
		}, 4},
		{"property named like a trait state", []protocol.BlockEntry{
			testDefinition("minecraft:parity_r5", nil),
			testDefinition("Minecraft:parity_r5", map[string]any{
				"traits": []any{map[string]any{
					"name": "minecraft:placement_position", "enabled_states": map[string]any{"vertical_half": uint8(1)},
				}},
				"properties": []any{property("minecraft:vertical_half")},
			}),
		}, 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			registry, err := NewNetworkBlockRegistry(test.entries)
			if err != nil {
				t.Fatal(err)
			}
			baseline, _ := NewNetworkBlockRegistry(nil)
			if got := registry.BlockCount() - baseline.BlockCount(); got != test.want {
				t.Fatalf("states = %d, want %d", got, test.want)
			}
		})
	}
}

// Of a trait state and a property sharing a name, the property names the value, as the client
// writes states by state ID and trait states have the lower ones.
func TestDigestNetworkBlocks_PropertyNamesValueOverSameNamedTraitState(t *testing.T) {
	registry, err := NewNetworkBlockRegistry([]protocol.BlockEntry{
		testDefinition("minecraft:parity_r6", map[string]any{"properties": []any{
			map[string]any{"name": "minecraft:vertical_half", "enum": []int32{0, 1}},
		}}),
		testDefinition("Minecraft:parity_r6", map[string]any{"traits": []any{map[string]any{
			"name": "minecraft:placement_position", "enabled_states": map[string]any{"vertical_half": uint8(1)},
		}}}),
	})
	if err != nil {
		t.Fatal(err)
	}
	first, ok := registry.StateToRuntimeID("minecraft:parity_r6", map[string]any{"minecraft:vertical_half": int32(0)})
	if !ok {
		t.Fatal("state missing")
	}
	var got []any
	for rid := first; rid < first+4; rid++ {
		_, properties, _ := registry.RuntimeIDToState(rid)
		got = append(got, properties["minecraft:vertical_half"])
	}
	if want := []any{int32(0), int32(1), int32(0), int32(1)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("values = %v, want %v", got, want)
	}
}
