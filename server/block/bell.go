package block

// Bell supplies block identity and actor metadata. Gameplay behaviour and
// the block-actor NBT codec are not implemented.
type Bell struct{}

// EncodeBlock returns the block identifier without selecting a gameplay state.
func (Bell) EncodeBlock() (string, map[string]any) { return "minecraft:bell", nil }

// BlockActorID returns the Bedrock NBT identifier for Bell.
func (Bell) BlockActorID() string { return "Bell" }
