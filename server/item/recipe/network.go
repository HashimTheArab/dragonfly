package recipe

import (
	"math"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// DecodeNetworkIngredient resolves an ordinary network ingredient. resolve must resolve
// the supplied item name and metadata; wildcard metadata is resolved using metadata 0.
// Unknown tags, unsupported descriptors and invalid counts or metadata fail closed.
// Invalid descriptors denote empty cells. The result uses the same variants marker
// as the embedded recipe catalog to represent wildcard metadata (32767).
func DecodeNetworkIngredient(in protocol.ItemDescriptorCount, resolve func(string, int16) (world.Item, bool)) (Item, bool) {
	if in.Count < 0 {
		return nil, false
	}
	switch d := in.Descriptor.(type) {
	case *protocol.InvalidItemDescriptor:
		if d == nil {
			return nil, false
		}
		return item.Stack{}, true
	case *protocol.DefaultItemDescriptor:
		if d == nil || resolve == nil || in.Count == 0 || d.Name == "" || d.MetadataValue < 0 || d.MetadataValue > math.MaxInt16 {
			return nil, false
		}
		meta := int16(d.MetadataValue)
		wildcard := meta == math.MaxInt16
		if wildcard {
			meta = 0
		}
		it, ok := resolve(d.Name, meta)
		if !ok || it == nil {
			return nil, false
		}
		name, actualMeta := it.EncodeItem()
		if name != d.Name || actualMeta != meta {
			return nil, false
		}
		st := item.NewStack(it, int(in.Count))
		if st.Empty() {
			return nil, false
		}
		if wildcard {
			st = st.WithValue("variants", true)
		}
		return st, true
	case *protocol.ItemTagItemDescriptor:
		if d == nil || in.Count == 0 || len(itemTags[d.Tag]) == 0 {
			return nil, false
		}
		return NewItemTag(d.Tag, int(in.Count)), true
	default:
		return nil, false
	}
}

// DecodeNetworkCraftingPattern decodes only ordinary crafting-table shaped or
// shapeless recipes with a single non-empty result. It retains that wire result without resolving it.
// r must be a *protocol.ShapedRecipe or *protocol.ShapelessRecipe. Dynamic,
// user-data, chemistry and other workstation recipes are unsupported.
func DecodeNetworkCraftingPattern(r any, resolve func(string, int16) (world.Item, bool)) (CraftingPattern, bool) {
	var input []protocol.ItemDescriptorCount
	var output []protocol.ItemStack
	var block string
	var shape Shape
	switch r := r.(type) {
	case *protocol.ShapedRecipe:
		if r == nil || r.Width < 1 || r.Width > 3 || r.Height < 1 || r.Height > 3 {
			return CraftingPattern{}, false
		}
		input, output, block = r.Input, r.Output, r.Block
		shape = NewShape(int(r.Width), int(r.Height))
	case *protocol.ShapelessRecipe:
		if r == nil {
			return CraftingPattern{}, false
		}
		input, output, block = r.Input, r.Output, r.Block
	default:
		return CraftingPattern{}, false
	}
	if block != "crafting_table" || len(output) != 1 || output[0].Count == 0 || output[0].NetworkID == 0 || len(input) == 0 || len(input) > 9 {
		return CraftingPattern{}, false
	}
	p := CraftingPattern{Input: make([]Item, len(input)), Shape: shape, Output: output}
	for i, in := range input {
		var ok bool
		p.Input[i], ok = DecodeNetworkIngredient(in, resolve)
		if !ok {
			return CraftingPattern{}, false
		}
	}
	if !p.valid() {
		return CraftingPattern{}, false
	}
	return p, true
}
