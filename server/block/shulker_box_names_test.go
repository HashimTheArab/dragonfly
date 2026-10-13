package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world"
)

func TestShulkerBoxNames_MatchesRegisteredFamily(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	registered := map[string]bool{}
	for _, b := range world.DefaultBlockRegistry.Blocks() {
		if box, ok := b.(ShulkerBox); ok {
			name, _ := box.EncodeBlock()
			registered[name] = true
		}
	}
	names := ShulkerBoxNames()
	for _, name := range names {
		if !registered[name] {
			t.Fatalf("unregistered or repeated shulker name %q", name)
		}
		delete(registered, name)
		if !IsShulkerBoxName(name) {
			t.Fatalf("registered shulker name %q was not classified", name)
		}
	}
	if len(registered) != 0 {
		t.Fatalf("missing registered shulker names: %v", registered)
	}
	names[0] = "modified"
	if ShulkerBoxNames()[0] == "modified" {
		t.Fatal("returned names shared mutable storage")
	}
}

func TestIsShulkerBoxName_UsesDeclaredFamily(t *testing.T) {
	for _, test := range []struct {
		name string
		want bool
	}{
		{name: "minecraft:undyed_shulker_box", want: true},
		{name: "purple_shulker_box", want: true},
		{name: "minecraft:shulker_box", want: true},
		{name: "shulker_box", want: true},
		{name: "custom:purple_shulker_box"},
		{name: "minecraft:unknown_shulker_box"},
		{name: "minecraft:chest"},
		{name: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := IsShulkerBoxName(test.name); got != test.want {
				t.Fatalf("IsShulkerBoxName(%q) = %t, want %t", test.name, got, test.want)
			}
		})
	}
}
