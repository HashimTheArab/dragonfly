package block

// Dispenser supplies block identity and actor metadata. Gameplay behaviour and
// the block-actor NBT codec are not implemented.
type Dispenser struct{}

// EncodeBlock returns the block identifier without selecting a gameplay state.
func (Dispenser) EncodeBlock() (string, map[string]any) { return "minecraft:dispenser", nil }

// BlockActorID returns the Bedrock NBT identifier for Dispenser.
func (Dispenser) BlockActorID() string { return "Dispenser" }
