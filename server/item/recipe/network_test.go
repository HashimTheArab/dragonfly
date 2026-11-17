package recipe

import (
	"math"
	"math/rand/v2"
	"slices"
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

// TestDecodeNetworkCraftingPattern_RespectsSymmetry checks that the advertised
// orientation policy survives decoding, including translated grids and batches.
func TestDecodeNetworkCraftingPattern_RespectsSymmetry(t *testing.T) {
	for _, symmetry := range []bool{false, true} {
		wire := protocol.ShapedRecipe{
			Width: 2, Height: 1, Block: "crafting_table", AssumeSymmetry: symmetry,
			Input:  []protocol.ItemDescriptorCount{networkTestIngredient(0, 1), networkTestIngredient(1, 2)},
			Output: []protocol.ItemStack{{ItemType: protocol.ItemType{NetworkID: 1}, Count: 1}},
		}
		pattern, ok := DecodeNetworkCraftingPattern(&wire, resolveGridTestItem)
		if !ok {
			t.Fatal("valid recipe rejected")
		}
		for _, width := range []int{2, 3} {
			for y := 0; y < width; y++ {
				for x := 0; x <= width-2; x++ {
					for _, mirror := range []bool{false, true} {
						grid := make([]item.Stack, width*width)
						first, second := gridStack("minecraft:oak_planks", 0, 2), gridStack("minecraft:oak_planks", 1, 4)
						if mirror {
							first, second = second, first
						}
						grid[y*width+x], grid[y*width+x+1] = first, second
						used, matched := pattern.Match(grid, 2)
						if want := !mirror || symmetry; matched != want {
							t.Fatalf("symmetry=%v mirror=%v width=%d x=%d y=%d: matched=%v want=%v", symmetry, mirror, width, x, y, matched, want)
						}
						if matched && (len(used) != 2 || used[0].Count != first.Count() || used[1].Count != second.Count()) {
							t.Fatalf("wrong batch consumption: %v", used)
						}
					}
				}
			}
		}
	}
}

// namedTestItems are the items resolveNamedTestItem knows. The planks are
// members of the vanilla "minecraft:planks" tag; the others are not.
var namedTestItems = []string{"minecraft:oak_planks", "minecraft:spruce_planks", "minecraft:stick", "minecraft:cobblestone"}

// resolveNamedTestItem resolves metadata 0 of namedTestItems, modelling a session palette.
func resolveNamedTestItem(name string, meta int16) (world.Item, bool) {
	if meta != 0 || !slices.Contains(namedTestItems, name) {
		return nil, false
	}
	return gridTestItem{name, 0}, true
}

func TestNetworkCraftingPatternMayMatch_Scope(t *testing.T) {
	stick := protocol.ItemDescriptorCount{Descriptor: &protocol.DefaultItemDescriptor{Name: "minecraft:stick"}, Count: 1}
	planks := protocol.ItemDescriptorCount{Descriptor: &protocol.ItemTagItemDescriptor{Tag: "minecraft:planks"}, Count: 1}
	empty := protocol.ItemDescriptorCount{Descriptor: &protocol.InvalidItemDescriptor{}}
	output := []protocol.ItemStack{{ItemType: protocol.ItemType{NetworkID: 1}, Count: 1}}
	shaped := func(block string, in ...protocol.ItemDescriptorCount) *protocol.ShapedRecipe {
		return &protocol.ShapedRecipe{Width: int32(len(in)), Height: 1, Input: in, Output: output, Block: block}
	}
	for _, tt := range []struct {
		name  string
		r     any
		names []string
		want  bool
	}{
		{name: "named ingredient present", r: shaped("crafting_table", stick, empty), names: []string{"minecraft:stick"}, want: true},
		{name: "named ingredient missing", r: shaped("crafting_table", stick), names: []string{"minecraft:cobblestone"}},
		{name: "one of two names missing", r: &protocol.ShapelessRecipe{Input: []protocol.ItemDescriptorCount{stick, planks}, Output: output, Block: "crafting_table"}, names: []string{"minecraft:oak_planks"}},
		{name: "tag member present", r: shaped("crafting_table", planks, stick), names: []string{"minecraft:stick", "minecraft:spruce_planks"}, want: true},
		{name: "tag member missing", r: shaped("crafting_table", planks), names: []string{"minecraft:stick"}},
		{name: "other workstation", r: shaped("stonecutter", stick), names: []string{"minecraft:stick"}},
		{name: "unsupported kind", r: &protocol.MultiRecipe{}, names: namedTestItems},
		{name: "unsupported descriptor", r: shaped("crafting_table", protocol.ItemDescriptorCount{Descriptor: &protocol.MoLangItemDescriptor{Expression: "true"}, Count: 1}), names: namedTestItems},
		{name: "typed nil recipe", r: (*protocol.ShapedRecipe)(nil), names: namedTestItems},
	} {
		if got := NetworkCraftingPatternMayMatch(tt.r, tt.names); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestNetworkCraftingPatternMayMatch_NeverRejectsAMatch checks the predicate
// against decoding and matching: whenever a recipe matches a grid, the
// predicate accepts the grid's item names.
func TestNetworkCraftingPatternMayMatch_NeverRejectsAMatch(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	output := []protocol.ItemStack{{ItemType: protocol.ItemType{NetworkID: 1}, Count: 1}}
	descriptor := func() protocol.ItemDescriptorCount {
		switch rng.IntN(6) {
		case 0:
			return protocol.ItemDescriptorCount{Descriptor: &protocol.ItemTagItemDescriptor{Tag: "minecraft:planks"}, Count: 1}
		case 1:
			return protocol.ItemDescriptorCount{Descriptor: &protocol.InvalidItemDescriptor{}}
		case 2:
			return protocol.ItemDescriptorCount{Descriptor: &protocol.DefaultItemDescriptor{Name: namedTestItems[rng.IntN(len(namedTestItems))], MetadataValue: math.MaxInt16}, Count: 1}
		default:
			return protocol.ItemDescriptorCount{Descriptor: &protocol.DefaultItemDescriptor{Name: namedTestItems[rng.IntN(len(namedTestItems))]}, Count: 1}
		}
	}
	matches, rejected := 0, 0
	for range 3000 {
		input := make([]protocol.ItemDescriptorCount, 1+rng.IntN(4))
		for i := range input {
			input[i] = descriptor()
		}
		var r any = &protocol.ShapelessRecipe{Input: input, Output: output, Block: "crafting_table"}
		if width := 1 + rng.IntN(2); rng.IntN(2) == 0 && len(input)%width == 0 && len(input)/width <= 2 {
			r = &protocol.ShapedRecipe{Width: int32(width), Height: int32(len(input) / width), Input: input, Output: output, Block: "crafting_table"}
		}
		pattern, ok := DecodeNetworkCraftingPattern(r, resolveNamedTestItem)
		grid := make([]item.Stack, 4)
		var names []string
		for range 1 + rng.IntN(3) {
			name := namedTestItems[rng.IntN(len(namedTestItems))]
			grid[rng.IntN(len(grid))] = item.NewStack(gridTestItem{name, 0}, 64)
		}
		for _, stack := range grid {
			if !stack.Empty() {
				name, _ := stack.Item().EncodeItem()
				names = append(names, name)
			}
		}
		may := NetworkCraftingPatternMayMatch(r, names)
		if !may {
			rejected++
		}
		if !ok {
			continue
		}
		if _, matched := pattern.Match(grid, 1); matched {
			matches++
			if !may {
				t.Fatalf("predicate rejected a matching recipe %+v for %v", r, names)
			}
		}
	}
	if matches < 50 || rejected < 500 {
		t.Fatalf("too few cases exercised: %d matches, %d rejections", matches, rejected)
	}
}
