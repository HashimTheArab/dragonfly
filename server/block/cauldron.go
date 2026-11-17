package block

// Cauldron supplies block identity and actor metadata. Gameplay behaviour and
// the block-actor NBT codec are not implemented.
type Cauldron struct{}

// EncodeBlock returns the block identifier without selecting a gameplay state.
func (Cauldron) EncodeBlock() (string, map[string]any) { return "minecraft:cauldron", nil }

// BlockActorID returns the Bedrock NBT identifier for Cauldron.
func (Cauldron) BlockActorID() string { return "Cauldron" }
