package block

import "github.com/df-mc/dragonfly/server/world"

// SnowLayer supplies state and item quantity for layered snow.
// It does not implement placement, melting or a world.Block model.
type SnowLayer struct {
	// Height is the encoded number of additional layers, from zero to seven.
	Height  int
	Covered bool
}

// EncodeBlock encodes the layered snow's palette state.
func (s SnowLayer) EncodeBlock() (string, map[string]any) {
	return "minecraft:snow_layer", map[string]any{"height": int32(s.Height), "covered_bit": boolByte(s.Covered)}
}

// EncodeItem returns the inventory identity of a snow layer.
func (SnowLayer) EncodeItem() (string, int16) { return "minecraft:snow_layer", 0 }

// PlacementItemCount counts the snow layers represented by this state.
func (s SnowLayer) PlacementItemCount() int { return s.Height + 1 }

// allSnowLayers enumerates the palette states with snow-layer item metadata.
func allSnowLayers() (all []world.BlockType) {
	for height := range 8 {
		for _, covered := range []bool{false, true} {
			all = append(all, SnowLayer{Height: height, Covered: covered})
		}
	}
	return
}
