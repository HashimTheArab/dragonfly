package block

// Lodestone supplies block identity and actor metadata. Gameplay behaviour and
// the block-actor NBT codec are not implemented.
type Lodestone struct{}

// EncodeBlock returns the block identifier without selecting a gameplay state.
func (Lodestone) EncodeBlock() (string, map[string]any) { return "minecraft:lodestone", nil }

// BlockActorID returns the Bedrock NBT identifier for Lodestone.
func (Lodestone) BlockActorID() string { return "Lodestone" }
