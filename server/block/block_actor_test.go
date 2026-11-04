package block_test

import (
	"math"
	"reflect"
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
		actor, ok := b.(world.BlockActor)
		if !ok {
			t.Fatalf("%T serializes an actor ID but has no BlockActorID method", b)
		}
		if got := actor.BlockActorID(); got != id {
			t.Fatalf("%T BlockActorID() = %q, want NBT ID %q", b, got, id)
		}
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
	registry.RegisterBlock(testBlockActor{name: "server:actor", id: "ServerActor", aliases: []string{"server:actor_alias"}})
	registry.RegisterBlock(testBlockActor{name: "server:alias", id: "AliasActor"})
	registry.Finalize()
	actors := registry.BlockActorIDs()
	actors["server:alias"] = "Wrong"
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
		for name, want := range map[string]string{"server:actor": "ServerActor", "server:actor_alias": "ServerActor", "server:alias": "AliasActor", "minecraft:chest": "Chest", "minecraft:bell": "Bell"} {
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
// one actor ID and rejects invalid registration without retaining partial metadata.
func TestBlockRegistry_ActorMetadataRejectsConflicts(t *testing.T) {
	for _, test := range []struct {
		name     string
		register func(world.BlockRegistry)
	}{
		{"empty name", func(r world.BlockRegistry) { r.RegisterBlock(testBlockActor{id: "Actor"}) }},
		{"empty ID", func(r world.BlockRegistry) { r.RegisterBlock(testBlockActor{name: "server:actor"}) }},
		{"registered conflict", func(r world.BlockRegistry) { r.RegisterBlock(testBlockActor{name: "minecraft:bell", id: "Wrong"}) }},
		{"implementation conflict", func(r world.BlockRegistry) { r.RegisterBlock(testBlockActor{name: "minecraft:chest", id: "Wrong"}) }},
		{"alias conflict", func(r world.BlockRegistry) {
			r.RegisterBlock(testBlockActor{name: "server:actor", id: "Actor", aliases: []string{"server:alias", "minecraft:chest"}})
		}},
		{"unregistered state", func(r world.BlockRegistry) { r.RegisterBlock(testImplementedActor{name: "server:actor"}) }},
		{"missing behaviour and metadata", func(r world.BlockRegistry) { r.RegisterBlock(testBlockIdentity{}) }},
		{"after finalization", func(r world.BlockRegistry) {
			r.Finalize()
			r.RegisterBlock(testBlockActor{name: "server:actor", id: "Actor"})
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			registry := world.NewBlockRegistry()
			func() {
				defer func() {
					if recover() == nil {
						t.Fatal("invalid actor metadata was accepted")
					}
				}()
				test.register(registry)
			}()
			registry.Finalize()
			for _, name := range []string{"server:actor", "server:alias"} {
				if id, ok := registry.BlockActorID(name); ok {
					t.Fatalf("failed registration retained %s metadata %q", name, id)
				}
			}
		})
	}
}

// TestBlockRegistry_MetadataOnlyTypesPreserveGameplay prevents actor metadata
// from adding palette states, codecs, models or block hashes.
func TestBlockRegistry_MetadataOnlyTypesPreserveGameplay(t *testing.T) {
	registry := world.NewBlockRegistry()
	registry.RegisterBlockState(world.BlockState{Name: "server:actor"})
	baseline := registry.(*world.BasicBlockRegistry).Clone()
	baseline.Finalize()
	for _, actor := range []world.BlockActor{block.Bell{}, block.Conduit{}, block.FlowerPot{}, block.ChiseledBookshelf{}} {
		if _, ok := actor.(world.Block); ok {
			t.Fatalf("metadata-only %T implements world.Block", actor)
		}
		if _, ok := actor.(world.NBTer); ok {
			t.Fatalf("metadata-only %T implements an NBT codec", actor)
		}
		if _, ok := actor.(interface{ Hash() (uint64, uint64) }); ok {
			t.Fatalf("metadata-only %T has a generated gameplay hash", actor)
		}
		if _, ok := actor.(interface{ Model() world.BlockModel }); ok {
			t.Fatalf("metadata-only %T supplies a gameplay model", actor)
		}
		registry.RegisterBlock(actor)
	}
	registry.RegisterBlock(testBlockActor{name: "server:actor", id: "ServerActor"})
	registry.RegisterBlock(testBlockActor{name: "server:alias", id: "AliasActor"})
	registry.Finalize()
	if registry.BlockCount() != baseline.BlockCount() {
		t.Fatal("metadata registration added block states")
	}
	for rid, before := range baseline.Blocks() {
		after, ok := registry.BlockByRuntimeID(uint32(rid))
		if !ok || !reflect.DeepEqual(after, before) {
			t.Fatalf("metadata registration replaced block at runtime ID %d", rid)
		}
		if registry.NBTBlock(uint32(rid)) != baseline.NBTBlock(uint32(rid)) {
			t.Fatalf("metadata registration changed NBT flag at runtime ID %d", rid)
		}
		beforeHash, _ := baseline.RuntimeIDToHash(uint32(rid))
		afterHash, _ := registry.RuntimeIDToHash(uint32(rid))
		if beforeHash != afterHash {
			t.Fatalf("metadata registration changed network hash at runtime ID %d", rid)
		}
	}
}

// TestBlockRegistry_ActorMetadataDoesNotEncodeNBT ensures registration reads
// type metadata without constructing or serializing gameplay state.
func TestBlockRegistry_ActorMetadataDoesNotEncodeNBT(t *testing.T) {
	registry := world.NewBlockRegistry()
	registry.RegisterBlockState(world.BlockState{Name: "server:actor"})
	registry.RegisterBlock(testImplementedActor{name: "server:actor"})
	registry.Finalize()
	if got, ok := registry.BlockActorID("server:actor"); !ok || got != "TestActor" {
		t.Fatalf("BlockActorID(server:actor) = %q, %v", got, ok)
	}
	rid, ok := registry.StateToRuntimeID("server:actor", nil)
	if !ok || !registry.NBTBlock(rid) {
		t.Fatal("implemented actor lost its NBT codec flag")
	}
}

// testBlockActor provides metadata without implementing gameplay behaviour.
type testBlockActor struct {
	name, id string
	aliases  []string
}

// EncodeBlock returns the fixture's block identifier.
func (a testBlockActor) EncodeBlock() (string, map[string]any) { return a.name, nil }

// BlockActorID returns the fixture's actor identifier.
func (a testBlockActor) BlockActorID() string { return a.id }

// BlockActorAliases returns the fixture's alternate block identifiers.
func (a testBlockActor) BlockActorAliases() []string { return a.aliases }

// testBlockIdentity has neither gameplay behaviour nor actor metadata.
type testBlockIdentity struct{}

// EncodeBlock returns the fixture's otherwise unsupported block identifier.
func (testBlockIdentity) EncodeBlock() (string, map[string]any) { return "server:actor", nil }

// testImplementedActor has an NBT codec that registration must never invoke.
type testImplementedActor struct {
	block.Stone
	name string
}

// EncodeBlock returns the fixture's registered state.
func (a testImplementedActor) EncodeBlock() (string, map[string]any) { return a.name, nil }

// Hash keeps the fixture out of the implementation hash table.
func (testImplementedActor) Hash() (uint64, uint64) { return 0, math.MaxUint64 }

// BlockActorID returns the fixture's actor identifier independently of its codec.
func (testImplementedActor) BlockActorID() string { return "TestActor" }

// EncodeNBT fails if registration tries to serialize gameplay state.
func (testImplementedActor) EncodeNBT() map[string]any { panic("registration must not encode NBT") }

// DecodeNBT preserves the fixture's gameplay implementation.
func (a testImplementedActor) DecodeNBT(map[string]any) any { return a }
