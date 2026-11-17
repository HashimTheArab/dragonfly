package block

// Crafter supplies block identity and actor metadata. Gameplay behaviour and
// the block-actor NBT codec are not implemented.
type Crafter struct{}

// EncodeBlock returns the block identifier without selecting a gameplay state.
func (Crafter) EncodeBlock() (string, map[string]any) { return "minecraft:crafter", nil }

// BlockActorID returns the Bedrock NBT identifier for Crafter.
func (Crafter) BlockActorID() string { return "Crafter" }
