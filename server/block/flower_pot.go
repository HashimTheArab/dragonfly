package block

// FlowerPot supplies block identity and actor metadata. Gameplay behaviour and
// the block-actor NBT codec are not implemented.
type FlowerPot struct{}

// EncodeBlock returns the block identifier without selecting a gameplay state.
func (FlowerPot) EncodeBlock() (string, map[string]any) { return "minecraft:flower_pot", nil }

// BlockActorID returns the Bedrock NBT identifier for FlowerPot.
func (FlowerPot) BlockActorID() string { return "FlowerPot" }
