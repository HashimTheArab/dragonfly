package item

import (
	"slices"
	"testing"

	"github.com/df-mc/dragonfly/server/world"
)

func TestReadNBT_DisplayLore(t *testing.T) {
	for _, test := range []struct {
		name string
		lore any
		want []string
	}{
		{name: "typed strings", lore: []string{"first", "second"}, want: []string{"first", "second"}},
		{name: "decoded strings", lore: []any{"first", "second"}, want: []string{"first", "second"}},
		{name: "compound list", lore: []any{map[string]any{"bad": int32(1)}}},
		{name: "mixed list", lore: []any{"first", nil, int32(1), map[string]any{}, "second"}, want: []string{"first", "second"}},
		{name: "wrong list type", lore: []int32{1}},
	} {
		for _, mode := range []struct {
			name string
			disk bool
		}{{name: "network"}, {name: "disk", disk: true}} {
			t.Run(test.name+"/"+mode.name, func(t *testing.T) {
				tag := map[string]any{"display": map[string]any{"Name": "Named bread", "Lore": test.lore}}
				stack := NewStack(Bread{}, 2)
				if mode.disk {
					stack = ReadNBT(map[string]any{"Name": "minecraft:bread", "Count": byte(2), "tag": tag}, nil)
				} else {
					stack = ReadNBT(tag, &stack)
				}
				if stack.Count() != 2 || stack.CustomName() != "Named bread" || !slices.Equal(stack.Lore(), test.want) {
					t.Fatalf("decoded display = count %d, name %q, lore %v; want 2, Named bread, %v", stack.Count(), stack.CustomName(), stack.Lore(), test.want)
				}
			})
		}
	}
}

// testEnchantment is registered locally because server/item/enchantment imports this
// package, so a test inside it cannot pull the real types in.
type testEnchantment struct{}

func (testEnchantment) Name() string                                   { return "Test" }
func (testEnchantment) MaxLevel() int                                  { return 5 }
func (testEnchantment) Cost(int) (int, int)                            { return 1, 10 }
func (testEnchantment) Rarity() EnchantmentRarity                      { return EnchantmentRarityCommon }
func (testEnchantment) CompatibleWithEnchantment(EnchantmentType) bool { return true }
func (testEnchantment) CompatibleWithItem(world.Item) bool             { return true }

// Level 0 reaches this from malformed or hostile NBT. Enchantments are 1-indexed, so it is
// clamped rather than stored as written.
func TestReadEnchantments_ClampsLevelBelowOne(t *testing.T) {
	const id = 9999
	RegisterEnchantment(id, testEnchantment{})

	stack := NewStack(Sword{Tier: ToolTierDiamond}, 1)
	readEnchantments(map[string]any{
		"ench": []any{
			map[string]any{"id": int16(id), "lvl": int16(0)},
		},
	}, &stack)

	enchants := stack.Enchantments()
	if len(enchants) != 1 {
		t.Fatalf("expected 1 enchantment, got %d", len(enchants))
	}
	if enchants[0].Level() != 1 {
		t.Fatalf("expected clamped enchantment level 1, got %d", enchants[0].Level())
	}
}

func TestReadDragonflyData_IgnoresMalformedData(t *testing.T) {
	stack := NewStack(Sword{Tier: ToolTierDiamond}, 1).WithValue("existing", "kept")
	readDragonflyData(map[string]any{"dragonflyData": []byte{0xff, 0x01}}, &stack)

	if got, ok := stack.Value("existing"); !ok || got != "kept" {
		t.Fatalf("existing stack value = %v, %t; want kept, true", got, ok)
	}
	if len(stack.Values()) != 1 {
		t.Fatalf("malformed dragonflyData changed stack values: %v", stack.Values())
	}
}
