package block

// Dropper supplies block identity and actor metadata. Gameplay behaviour and
// the block-actor NBT codec are not implemented.
type Dropper struct{}

// EncodeBlock returns the block identifier without selecting a gameplay state.
func (Dropper) EncodeBlock() (string, map[string]any) { return "minecraft:dropper", nil }

// BlockActorID returns the Bedrock NBT identifier for Dropper.
func (Dropper) BlockActorID() string { return "Dropper" }
