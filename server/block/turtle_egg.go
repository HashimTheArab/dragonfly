package block

import "github.com/df-mc/dragonfly/server/world"

// TurtleEgg supplies state and item quantity for a cluster of turtle eggs.
// It does not implement hatching, placement or a world.Block model.
type TurtleEgg struct {
	// AdditionalEggs ranges from zero to three.
	AdditionalEggs int
	// Cracks ranges from zero (uncracked) to two (maximally cracked).
	Cracks int
}

// EncodeBlock encodes the egg cluster's palette state.
func (t TurtleEgg) EncodeBlock() (string, map[string]any) {
	return "minecraft:turtle_egg", map[string]any{
		"turtle_egg_count": [...]string{"one_egg", "two_egg", "three_egg", "four_egg"}[t.AdditionalEggs],
		"cracked_state":    [...]string{"no_cracks", "cracked", "max_cracked"}[t.Cracks],
	}
}

// EncodeItem returns the inventory identity of a turtle egg.
func (TurtleEgg) EncodeItem() (string, int16) { return "minecraft:turtle_egg", 0 }

// PlacementItemCount counts the eggs represented by this state.
func (t TurtleEgg) PlacementItemCount() int { return t.AdditionalEggs + 1 }

// allTurtleEggs enumerates the palette states with turtle-egg item metadata.
func allTurtleEggs() (all []world.BlockType) {
	for additional := range 4 {
		for cracks := range 3 {
			all = append(all, TurtleEgg{AdditionalEggs: additional, Cracks: cracks})
		}
	}
	return
}
