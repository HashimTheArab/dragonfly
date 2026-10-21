package recipe

import (
	"math"
	"testing"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// gridTestItem provides concrete metadata variants without relying on world registration.
type gridTestItem struct {
	name string
	meta int16
}

// EncodeItem returns the test item's identity and metadata.
func (i gridTestItem) EncodeItem() (string, int16) { return i.name, i.meta }

// resolveGridTestItem recognises only two concrete variants, modelling a checked item resolver.
func resolveGridTestItem(name string, meta int16) (world.Item, bool) {
	if name != "minecraft:oak_planks" || meta < 0 || meta > 1 {
		return nil, false
	}
	return gridTestItem{name, meta}, true
}

// networkTestIngredient describes a concrete or wildcard network ingredient.
func networkTestIngredient(meta, count int32) protocol.ItemDescriptorCount {
	return protocol.ItemDescriptorCount{Descriptor: &protocol.DefaultItemDescriptor{Name: "minecraft:oak_planks", MetadataValue: meta}, Count: count}
}

func TestDecodeNetworkIngredient_CheckedDescriptors(t *testing.T) {
	for _, tt := range []struct {
		name                string
		in                  protocol.ItemDescriptorCount
		ok, empty, wildcard bool
	}{
		{name: "invalid is empty", in: protocol.ItemDescriptorCount{Descriptor: &protocol.InvalidItemDescriptor{}}, ok: true, empty: true},
		{name: "invalid positive count stays empty", in: protocol.ItemDescriptorCount{Descriptor: &protocol.InvalidItemDescriptor{}, Count: 1}, ok: true, empty: true},
		{name: "exact", in: networkTestIngredient(1, 2), ok: true},
		{name: "wildcard", in: networkTestIngredient(math.MaxInt16, 2), ok: true, wildcard: true},
		{name: "tag", in: protocol.ItemDescriptorCount{Descriptor: &protocol.ItemTagItemDescriptor{Tag: "minecraft:planks"}, Count: 2}, ok: true},
		{name: "nil descriptor", in: protocol.ItemDescriptorCount{}},
		{name: "typed nil default", in: protocol.ItemDescriptorCount{Descriptor: (*protocol.DefaultItemDescriptor)(nil), Count: 1}},
		{name: "typed nil invalid", in: protocol.ItemDescriptorCount{Descriptor: (*protocol.InvalidItemDescriptor)(nil)}},
		{name: "typed nil tag", in: protocol.ItemDescriptorCount{Descriptor: (*protocol.ItemTagItemDescriptor)(nil), Count: 1}},
		{name: "negative count", in: networkTestIngredient(0, -1)},
		{name: "negative invalid count", in: protocol.ItemDescriptorCount{Descriptor: &protocol.InvalidItemDescriptor{}, Count: -1}},
		{name: "zero concrete count", in: networkTestIngredient(0, 0)},
		{name: "negative metadata", in: networkTestIngredient(-1, 1)},
		{name: "oversized metadata", in: networkTestIngredient(math.MaxInt16+1, 1)},
		{name: "unsupported metadata", in: networkTestIngredient(2, 1)},
		{name: "unknown name", in: protocol.ItemDescriptorCount{Descriptor: &protocol.DefaultItemDescriptor{Name: "unknown"}, Count: 1}},
		{name: "unknown tag", in: protocol.ItemDescriptorCount{Descriptor: &protocol.ItemTagItemDescriptor{Tag: "unknown"}, Count: 1}},
		{name: "empty tag", in: protocol.ItemDescriptorCount{Descriptor: &protocol.ItemTagItemDescriptor{}, Count: 1}},
		{name: "zero tag count", in: protocol.ItemDescriptorCount{Descriptor: &protocol.ItemTagItemDescriptor{Tag: "minecraft:planks"}}},
		{name: "molang", in: protocol.ItemDescriptorCount{Descriptor: &protocol.MoLangItemDescriptor{Expression: "true"}, Count: 1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := DecodeNetworkIngredient(tt.in, resolveGridTestItem)
			if ok != tt.ok {
				t.Fatalf("ok=%v, want %v", ok, tt.ok)
			}
			if !ok {
				if got != nil {
					t.Fatal("failure returned ingredient")
				}
				return
			}
			if got.Empty() != tt.empty {
				t.Fatalf("empty=%v, want %v", got.Empty(), tt.empty)
			}
			if st, ok := got.(item.Stack); ok && !st.Empty() {
				_, wildcard := st.Value("variants")
				if wildcard != tt.wildcard {
					t.Fatalf("wildcard=%v, want %v", wildcard, tt.wildcard)
				}
				if st.Count() != int(tt.in.Count) {
					t.Fatalf("count=%v", st.Count())
				}
				if tt.wildcard {
					has := item.NewStack(gridTestItem{"minecraft:oak_planks", 1}, 2)
					if match, ok := MatchingStacks(has, st); !match || !ok {
						t.Fatal("wildcard did not match metadata variant")
					}
				}
			}
		})
	}
	for _, resolve := range []func(string, int16) (world.Item, bool){nil, func(string, int16) (world.Item, bool) { return nil, true }, func(string, int16) (world.Item, bool) { return gridTestItem{"wrong", 0}, true }, func(string, int16) (world.Item, bool) { return gridTestItem{"minecraft:oak_planks", 0}, true }} {
		if _, ok := DecodeNetworkIngredient(networkTestIngredient(1, 1), resolve); ok {
			t.Fatal("accepted absent or inconsistent resolution")
		}
	}
}

func TestDecodeNetworkCraftingPattern_RestrictsScope(t *testing.T) {
	output := []protocol.ItemStack{{ItemType: protocol.ItemType{NetworkID: 1}, Count: 1}}
	ingredient := networkTestIngredient(0, 1)
	shaped := protocol.ShapedRecipe{Width: 1, Height: 1, Input: []protocol.ItemDescriptorCount{ingredient}, Output: output, Block: "crafting_table"}
	shapeless := protocol.ShapelessRecipe{Input: []protocol.ItemDescriptorCount{ingredient}, Output: output, Block: "crafting_table"}
	for _, r := range []any{&shaped, &shapeless} {
		p, ok := DecodeNetworkCraftingPattern(r, resolveGridTestItem)
		if !ok || len(p.Input) != 1 || len(p.Output) != 1 || p.Output[0].NetworkID != 1 {
			t.Fatalf("ordinary recipe rejected or lost result: %+v, %v", p, ok)
		}
		grid := make([]item.Stack, 9)
		grid[8] = item.NewStack(gridTestItem{"minecraft:oak_planks", 0}, 1)
		if _, ok := p.Match(grid, 1); !ok {
			t.Fatal("decoded recipe did not match grid")
		}
	}
	for _, tt := range []struct {
		name string
		edit func(*protocol.ShapedRecipe)
	}{
		{"zero width", func(r *protocol.ShapedRecipe) { r.Width = 0 }},
		{"oversized width", func(r *protocol.ShapedRecipe) { r.Width = 4 }},
		{"negative height", func(r *protocol.ShapedRecipe) { r.Height = -1 }},
		{"wrong cell count", func(r *protocol.ShapedRecipe) { r.Width = 2 }},
		{"different station", func(r *protocol.ShapedRecipe) { r.Block = "stonecutter" }},
		{"missing output", func(r *protocol.ShapedRecipe) { r.Output = nil }},
		{"multiple outputs", func(r *protocol.ShapedRecipe) { r.Output = append(r.Output, r.Output[0]) }},
		{"empty output", func(r *protocol.ShapedRecipe) {
			r.Output = []protocol.ItemStack{{ItemType: protocol.ItemType{NetworkID: 1}}}
		}},
		{"air output", func(r *protocol.ShapedRecipe) { r.Output = []protocol.ItemStack{{Count: 1}} }},
		{"empty pattern", func(r *protocol.ShapedRecipe) {
			r.Input = []protocol.ItemDescriptorCount{{Descriptor: &protocol.InvalidItemDescriptor{}}}
		}},
		{"unknown descriptor", func(r *protocol.ShapedRecipe) {
			r.Input = []protocol.ItemDescriptorCount{{Descriptor: &protocol.MoLangItemDescriptor{}, Count: 1}}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := shaped
			tt.edit(&r)
			if _, ok := DecodeNetworkCraftingPattern(&r, resolveGridTestItem); ok {
				t.Fatal("accepted unsupported or malformed recipe")
			}
		})
	}
	for _, r := range []any{nil, (*protocol.ShapedRecipe)(nil), (*protocol.ShapelessRecipe)(nil), &protocol.UserDataShapelessRecipe{ShapelessRecipe: shapeless}, &protocol.ShapedChemistryRecipe{ShapedRecipe: shaped}, &protocol.ShapelessChemistryRecipe{ShapelessRecipe: shapeless}, &protocol.SmithingTransformRecipe{}, &protocol.MultiRecipe{}} {
		if _, ok := DecodeNetworkCraftingPattern(r, resolveGridTestItem); ok {
			t.Fatalf("accepted %T", r)
		}
	}
}
