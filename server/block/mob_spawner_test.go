package block_test

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

func TestMobSpawner(t *testing.T) {
	spawner := block.MobSpawner{}
	name, properties := spawner.EncodeBlock()
	if name != "minecraft:mob_spawner" || properties != nil {
		t.Fatalf("EncodeBlock() = (%q, %v), want mob spawner with no properties", name, properties)
	}
	if name, meta := spawner.EncodeItem(); name != "minecraft:mob_spawner" || meta != 0 {
		t.Fatalf("EncodeItem() = (%q, %d), want (minecraft:mob_spawner, 0)", name, meta)
	}
	if _, ok := spawner.Model().(model.Solid); !ok {
		t.Fatalf("Model() = %T, want model.Solid", spawner.Model())
	}

	info := spawner.BreakInfo()
	pickaxe := item.Pickaxe{Tier: item.ToolTierWood}
	if info.Hardness != 5 || info.BlastResistance != 25 || !info.Harvestable(pickaxe) || !info.Effective(pickaxe) {
		t.Fatalf("BreakInfo() = %+v, want hardness 5, resistance 25, and pickaxe mining", info)
	}
	if drops := info.Drops(pickaxe, nil); len(drops) != 0 {
		t.Fatalf("BreakInfo().Drops() = %v, want no item drops", drops)
	}

	if nbt := spawner.EncodeNBT(); nbt["id"] != "MobSpawner" {
		t.Fatalf("EncodeNBT() = %v, want MobSpawner id", nbt)
	}
	if _, ok := spawner.DecodeNBT(map[string]any{"id": "MobSpawner"}).(block.MobSpawner); !ok {
		t.Fatalf("DecodeNBT() returned unexpected type")
	}
	if registered, ok := world.ItemByName("minecraft:mob_spawner", 0); !ok {
		t.Fatal("mob spawner item is not registered")
	} else if _, ok := registered.(block.MobSpawner); !ok {
		t.Fatalf("registered item = %T, want block.MobSpawner", registered)
	}
}
