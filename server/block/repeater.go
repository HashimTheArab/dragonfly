package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// Repeater supplies state and item identity for a redstone repeater.
// It does not implement redstone behaviour or a world.Block model.
type Repeater struct {
	Facing cube.Direction
	// Delay is the encoded delay, from zero to three.
	Delay   int
	Powered bool
}

// EncodeBlock encodes the repeater's palette state.
func (r Repeater) EncodeBlock() (string, map[string]any) {
	name := "minecraft:unpowered_repeater"
	if r.Powered {
		name = "minecraft:powered_repeater"
	}
	return name, map[string]any{"minecraft:cardinal_direction": r.Facing.String(), "repeater_delay": int32(r.Delay)}
}

// EncodeItem returns the shared item identity of powered and unpowered states.
func (Repeater) EncodeItem() (string, int16) { return "minecraft:repeater", 0 }

// allRepeaters enumerates the palette states with repeater item metadata.
func allRepeaters() (all []world.BlockType) {
	for _, facing := range cube.Directions() {
		for delay := range 4 {
			for _, powered := range []bool{false, true} {
				all = append(all, Repeater{Facing: facing, Delay: delay, Powered: powered})
			}
		}
	}
	return
}
