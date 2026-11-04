package block

// Conduit supplies block identity and actor metadata. Gameplay behaviour and
// the block-actor NBT codec are not implemented.
type Conduit struct{}

// EncodeBlock returns the block identifier without selecting a gameplay state.
func (Conduit) EncodeBlock() (string, map[string]any) { return "minecraft:conduit", nil }

// BlockActorID returns the Bedrock NBT identifier for Conduit.
func (Conduit) BlockActorID() string { return "Conduit" }
