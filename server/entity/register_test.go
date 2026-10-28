package entity

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world"
)

func TestDefaultRegistryExcludesPassiveAnimals(t *testing.T) {
	for _, name := range []string{"minecraft:cow", "minecraft:pig", "minecraft:sheep", "minecraft:chicken"} {
		t.Run(name, func(t *testing.T) {
			if _, ok := DefaultRegistry.Lookup(name); ok {
				t.Fatalf("default registry implements experimental animal %s", name)
			}
		})
	}
	for _, name := range []string{"minecraft:item", "minecraft:tnt", "minecraft:arrow"} {
		t.Run(name, func(t *testing.T) {
			if _, ok := DefaultRegistry.Lookup(name); !ok {
				t.Fatalf("default registry lost supported entity %s", name)
			}
		})
	}
}

func TestDefaultRegistryPlacementPassableTypes(t *testing.T) {
	passable := map[string]bool{"minecraft:item": true, "minecraft:arrow": true, "minecraft:xp_orb": true}
	for _, typ := range DefaultRegistry.Types() {
		t.Run(typ.EncodeEntity(), func(t *testing.T) {
			p, ok := typ.(world.PlacementPassableType)
			if got := ok && p.PassableByPlacement(); got != passable[typ.EncodeEntity()] {
				t.Fatalf("passable by placement = %v, want %v", got, passable[typ.EncodeEntity()])
			}
		})
	}
}
