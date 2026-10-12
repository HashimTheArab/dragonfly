package block_test

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
)

// TestBlockRegistry_ActorIDsMatchImplementedNBT checks every implementation,
// including new block types, against the ID it actually serializes.
func TestBlockRegistry_ActorIDsMatchImplementedNBT(t *testing.T) {
	registry := world.NewBlockRegistry()
	registry.Finalize()
	count := 0
	for _, b := range registry.Blocks() {
		nbter, ok := b.(world.NBTer)
		if !ok {
			continue
		}
		id, ok := nbter.EncodeNBT()["id"].(string)
		if !ok || id == "" {
			continue
		}
		count++
		name, _ := b.EncodeBlock()
		if got, ok := registry.BlockActorID(name); !ok || got != id {
			t.Fatalf("BlockActorID(%q) = %q, %v, want NBT ID %q", name, got, ok, id)
		}
	}
	if count == 0 {
		t.Fatal("no implemented block actors were checked")
	}
}

// TestBlockRegistry_ActorMetadataCoversUnimplementedBlocks preserves actor
// aliases and catalog-derived chest variants without inventing block behaviour.
func TestBlockRegistry_ActorMetadataCoversUnimplementedBlocks(t *testing.T) {
	registry := world.NewBlockRegistry()
	registry.Finalize()
	for name, want := range map[string]string{
		"minecraft:trapped_chest":      "Chest",
		"minecraft:skull":              "Skull",
		"minecraft:mob_spawner":        "MobSpawner",
		"minecraft:bell":               "Bell",
		"minecraft:conduit":            "Conduit",
		"minecraft:flower_pot":         "FlowerPot",
		"minecraft:chiseled_bookshelf": "ChiseledBookshelf",
	} {
		if got, ok := registry.BlockActorID(name); !ok || got != want {
			t.Errorf("BlockActorID(%q) = %q, %v, want %q", name, got, ok, want)
		}
	}
	chests := block.CopperChestNames()
	if len(chests) != 8 {
		t.Fatalf("copper chest variant count = %d, want 8", len(chests))
	}
	for _, name := range chests {
		if got, ok := registry.BlockActorID(name); !ok || got != "Chest" {
			t.Errorf("BlockActorID(%q) = %q, %v, want Chest", name, got, ok)
		}
	}
	for _, name := range []string{"minecraft:stone", "minecraft:copper_chestplate", "server:missing", ""} {
		if got, ok := registry.BlockActorID(name); ok || got != "" {
			t.Errorf("BlockActorID(%q) = %q, %v, want absent", name, got, ok)
		}
	}
}

// TestBlockRegistry_ActorMetadataSurvivesCloneAndSnapshot keeps custom palette
// metadata independent from the vanilla registry and stable on replay.
func TestBlockRegistry_ActorMetadataSurvivesCloneAndSnapshot(t *testing.T) {
	registry := world.NewBlockRegistry()
	registry.RegisterBlockState(world.BlockState{Name: "server:actor"})
	registry.RegisterBlockActor("server:actor", "ServerActor")
	registry.Finalize()
	clone := registry.(*world.BasicBlockRegistry).Clone()
	data, err := world.MarshalBlockRegistry(clone)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := world.UnmarshalBlockRegistry(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []world.BlockRegistry{registry, clone, restored} {
		for name, want := range map[string]string{"server:actor": "ServerActor", "minecraft:chest": "Chest", "minecraft:bell": "Bell"} {
			if got, ok := r.BlockActorID(name); !ok || got != want {
				t.Fatalf("BlockActorID(%q) = %q, %v, want %q", name, got, ok, want)
			}
		}
	}
	if _, ok := world.DefaultBlockRegistry.BlockActorID("server:actor"); ok {
		t.Fatal("custom metadata leaked into the default registry")
	}
}

// TestBlockRegistry_ActorMetadataRejectsConflicts protects a block identifier's
// one actor ID, including disagreement with its implemented NBT encoding.
func TestBlockRegistry_ActorMetadataRejectsConflicts(t *testing.T) {
	for _, test := range []struct {
		name     string
		register func(world.BlockRegistry)
	}{
		{"empty name", func(r world.BlockRegistry) { r.RegisterBlockActor("", "Actor") }},
		{"empty ID", func(r world.BlockRegistry) { r.RegisterBlockActor("server:actor", "") }},
		{"registered conflict", func(r world.BlockRegistry) { r.RegisterBlockActor("minecraft:bell", "Wrong") }},
		{"implementation conflict", func(r world.BlockRegistry) { r.RegisterBlockActor("minecraft:chest", "Wrong"); r.Finalize() }},
		{"after finalization", func(r world.BlockRegistry) { r.Finalize(); r.RegisterBlockActor("server:actor", "Actor") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid actor metadata was accepted")
				}
			}()
			test.register(world.NewBlockRegistry())
		})
	}
}
