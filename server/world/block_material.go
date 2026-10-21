package world

// PlacementItemCounter reports how many inventory items a placed block part
// represents. Zero means another part accounts for the item, or the block is air.
// Blocks without this capability represent one item. This is not a drop count.
type PlacementItemCounter interface {
	PlacementItemCount() int
}

// PlacementItem returns the inventory item and quantity represented by b.
// A nil item leaves the block as an unidentified material. Liquids remain block
// quantities, because flowing cells do not correspond to individual buckets.
// Metadata-only registrations supply item semantics without block behaviour.
func PlacementItem(b Block) (Item, int) {
	if _, liquid := b.(Liquid); liquid {
		return nil, 1
	}
	var source any = b
	if unknown, ok := b.(unknownBlock); ok && unknown.placementItem != nil {
		source = unknown.placementItem
	}
	count := 1
	if counter, ok := source.(PlacementItemCounter); ok {
		count = counter.PlacementItemCount()
	}
	if it, ok := source.(Item); ok {
		return it, count
	}
	name, _ := b.EncodeBlock()
	it, _ := ItemByName(name, 0)
	return it, count
}
