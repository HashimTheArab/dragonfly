package java

import (
	"fmt"
	"maps"
	"slices"
	"testing"

	_ "github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
)

// TestResolve preserves Java orientation and supplies declared defaults for omitted properties.
func TestResolve(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	for _, test := range []struct {
		input, name string
		key         string
		value       any
	}{
		{"oak_stairs[half=top,facing=west]", "minecraft:oak_stairs", "upside_down_bit", true},
		{"oak_log[axis=x]", "minecraft:oak_log", "pillar_axis", "x"},
		{"grass", "minecraft:short_grass", "", nil},
		{"grass_block", "minecraft:grass_block", "", nil},
	} {
		state, err := Parse(test.input)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Resolve(world.DefaultBlockRegistry, state)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != test.name || test.key != "" && got.Properties[test.key] != test.value {
			t.Errorf("%s -> %+v", test.input, got)
		}
	}
	for _, input := range []string{"mod:missing", "oak_stairs[facing=sideways]", "oak_log[typo=x]"} {
		state, err := Parse(input)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Resolve(world.DefaultBlockRegistry, state); err == nil {
			t.Errorf("accepted unsupported %s", input)
		}
	}
}

// TestLegacy preserves metadata and rejects unknown extended IDs instead of turning them into air.
func TestLegacy(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	got, err := Legacy(world.DefaultBlockRegistry, 35, 14)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "minecraft:red_wool" {
		t.Fatalf("legacy red wool = %+v", got)
	}
	if _, err := Legacy(world.DefaultBlockRegistry, 256, 0); err == nil {
		t.Fatal("accepted unmapped extended ID")
	}
}

// TestLegacy_MigratesPreFlatteningProperties converts the property syntax the
// legacy table still records for cauldrons and walls.
func TestLegacy_MigratesPreFlatteningProperties(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	for _, test := range []struct {
		id    uint16
		meta  uint8
		name  string
		key   string
		value any
	}{
		{118, 0, "minecraft:cauldron", "fill_level", int32(0)},
		{118, 3, "minecraft:cauldron", "fill_level", int32(6)},
		{139, 0, "minecraft:cobblestone_wall", "wall_connection_type_north", "none"},
		{139, 1, "minecraft:mossy_cobblestone_wall", "wall_connection_type_north", "none"},
	} {
		got, err := Legacy(world.DefaultBlockRegistry, test.id, test.meta)
		if err != nil {
			t.Fatalf("%d:%d: %v", test.id, test.meta, err)
		}
		if got.Name != test.name || got.Properties[test.key] != test.value {
			t.Errorf("%d:%d = %+v, want %s %s=%v", test.id, test.meta, got, test.name, test.key, test.value)
		}
	}
}

// TestLegacy_ResolvesEveryTableEntry guards the whole legacy table: an entry the
// converter cannot resolve aborts a schematic import, so none may exist.
func TestLegacy_ResolvesEveryTableEntry(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	data, err := loadMapping()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range slices.Sorted(maps.Keys(data.Legacy)) {
		var id uint16
		var meta uint8
		if _, err := fmt.Sscanf(key, "%d:%d", &id, &meta); err != nil {
			t.Fatalf("legacy key %q: %v", key, err)
		}
		if _, err := Legacy(world.DefaultBlockRegistry, id, meta); err != nil {
			t.Errorf("%s -> %s: %v", key, data.Legacy[key], err)
		}
	}
}
