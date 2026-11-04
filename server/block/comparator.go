package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// Comparator supplies state and item identity for a redstone comparator.
// It does not implement redstone behaviour or a world.Block model.
type Comparator struct {
	Facing    cube.Direction
	Powered   bool
	OutputLit bool
	Subtract  bool
}

// EncodeBlock encodes the comparator's palette state.
func (c Comparator) EncodeBlock() (string, map[string]any) {
	name := "minecraft:unpowered_comparator"
	if c.Powered {
		name = "minecraft:powered_comparator"
	}
	return name, map[string]any{
		"minecraft:cardinal_direction": c.Facing.String(),
		"output_lit_bit":               boolByte(c.OutputLit),
		"output_subtract_bit":          boolByte(c.Subtract),
	}
}

// EncodeItem returns the shared item identity of powered and unpowered states.
func (Comparator) EncodeItem() (string, int16) { return "minecraft:comparator", 0 }

// allComparators enumerates the palette states with comparator item metadata.
func allComparators() (all []world.BlockType) {
	for _, facing := range cube.Directions() {
		for _, powered := range []bool{false, true} {
			for _, lit := range []bool{false, true} {
				for _, subtract := range []bool{false, true} {
					all = append(all, Comparator{Facing: facing, Powered: powered, OutputLit: lit, Subtract: subtract})
				}
			}
		}
	}
	return
}
