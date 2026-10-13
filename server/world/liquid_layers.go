package world

// LiquidFromLayers resolves the first liquid in the primary and secondary
// Bedrock block layers. read must distinguish unavailable blocks from air.
// An unavailable layer stops the query; a primary liquid never reads secondary.
func LiquidFromLayers(read func(layer uint8) (Block, bool)) (Liquid, bool) {
	for layer := uint8(0); layer < 2; layer++ {
		b, known := read(layer)
		if !known {
			return nil, false
		}
		if liquid, ok := b.(Liquid); ok {
			return liquid, true
		}
	}
	return nil, false
}
