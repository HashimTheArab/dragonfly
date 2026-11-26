package item

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world"
)

func TestDurable_BreaksOnVanillaLastUse(t *testing.T) {
	maxDamage := map[string]int{
		"minecraft:bow":                  384,
		"minecraft:crossbow":             464,
		"minecraft:elytra":               432,
		"minecraft:fishing_rod":          384,
		"minecraft:flint_and_steel":      64,
		"minecraft:mace":                 500,
		"minecraft:shears":               238,
		"minecraft:shield":               336,
		"minecraft:turtle_helmet":        275,
		"minecraft:leather_helmet":       55,
		"minecraft:leather_chestplate":   80,
		"minecraft:leather_leggings":     75,
		"minecraft:leather_boots":        65,
		"minecraft:golden_helmet":        77,
		"minecraft:golden_chestplate":    112,
		"minecraft:golden_leggings":      105,
		"minecraft:golden_boots":         91,
		"minecraft:copper_helmet":        121,
		"minecraft:copper_chestplate":    176,
		"minecraft:copper_leggings":      165,
		"minecraft:copper_boots":         143,
		"minecraft:chainmail_helmet":     165,
		"minecraft:chainmail_chestplate": 240,
		"minecraft:chainmail_leggings":   225,
		"minecraft:chainmail_boots":      195,
		"minecraft:iron_helmet":          165,
		"minecraft:iron_chestplate":      240,
		"minecraft:iron_leggings":        225,
		"minecraft:iron_boots":           195,
		"minecraft:diamond_helmet":       363,
		"minecraft:diamond_chestplate":   528,
		"minecraft:diamond_leggings":     495,
		"minecraft:diamond_boots":        429,
		"minecraft:netherite_helmet":     407,
		"minecraft:netherite_chestplate": 592,
		"minecraft:netherite_leggings":   555,
		"minecraft:netherite_boots":      481,
	}
	for tier, damage := range map[string]int{"wooden": 59, "golden": 32, "stone": 131, "copper": 190, "iron": 250, "diamond": 1561, "netherite": 2031} {
		for _, tool := range []string{"pickaxe", "axe", "shovel", "sword", "hoe"} {
			maxDamage["minecraft:"+tier+"_"+tool] = damage
		}
	}

	covered := make(map[string]bool, len(maxDamage))
	for _, it := range world.Items() {
		durable, ok := it.(Durable)
		if !ok {
			continue
		}
		name, _ := it.EncodeItem()
		covered[name] = true
		t.Run(name, func(t *testing.T) {
			want, ok := maxDamage[name]
			if !ok {
				t.Fatal("durable item has no vanilla maximum damage")
			}
			s := NewStack(it, 1)
			for use := 1; use <= want; use++ {
				if s = s.Damage(1); s.Empty() {
					t.Fatalf("broke on use %d, want it to last %d uses", use, want+1)
				}
			}
			if got := WriteNBT(s, false)["Damage"]; got != int32(want) {
				t.Fatalf("last usable state has damage %v, want %d", got, want)
			}
			s = s.Damage(1)
			if durable.DurabilityInfo().Persistent {
				if s.Empty() || WriteNBT(s, false)["Damage"] != int32(want) {
					t.Fatalf("persistent item changed past its last usable state: empty=%v damage=%v", s.Empty(), WriteNBT(s, false)["Damage"])
				}
				return
			}
			if !s.Empty() {
				t.Fatalf("survived use %d, want it to break", want+1)
			}
		})
	}
	for name := range maxDamage {
		if !covered[name] {
			t.Errorf("%s is not a registered durable item", name)
		}
	}
}
