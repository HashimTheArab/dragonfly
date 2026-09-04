package java

import (
	_ "github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
	"testing"
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
