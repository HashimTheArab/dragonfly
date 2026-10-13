package block

// ChiseledBookshelf supplies block identity and actor metadata. Gameplay behaviour and
// the block-actor NBT codec are not implemented.
type ChiseledBookshelf struct{}

// EncodeBlock returns the block identifier without selecting a gameplay state.
func (ChiseledBookshelf) EncodeBlock() (string, map[string]any) {
	return "minecraft:chiseled_bookshelf", nil
}

// BlockActorID returns the Bedrock NBT identifier for ChiseledBookshelf.
func (ChiseledBookshelf) BlockActorID() string { return "ChiseledBookshelf" }
