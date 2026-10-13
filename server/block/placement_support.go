package block

import "github.com/df-mc/dragonfly/server/world"

// NormaliseLayerBlock validates an exact Bedrock layer write. Nil clears the
// layer to air. Primary blocks use layer zero; layer one accepts liquids or air.
func NormaliseLayerBlock(b world.Block, layer uint8) (world.Block, bool) {
	if layer > 1 {
		return nil, false
	}
	if b == nil {
		b = Air{}
	}
	if layer == 0 {
		return b, true
	}
	switch b.(type) {
	case world.Liquid, Air:
		return b, true
	default:
		return nil, false
	}
}

// CanClickToPlace reports whether b can be clicked to place an adjacent block.
// It excludes air, activatable blocks and replaceable blocks whose interaction
// would instead place in the clicked cell.
func CanClickToPlace(b world.Block) bool {
	if b == nil {
		return false
	}
	switch b.(type) {
	case Air, Activatable, Replaceable:
		return false
	default:
		return true
	}
}
