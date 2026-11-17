package block

// SculkSensor supplies block identity and actor metadata. Gameplay behaviour
// and the block-actor NBT codec are not implemented.
type SculkSensor struct{}

// EncodeBlock returns the block identifier without selecting a gameplay state.
func (SculkSensor) EncodeBlock() (string, map[string]any) { return "minecraft:sculk_sensor", nil }

// BlockActorID returns the Bedrock NBT identifier for SculkSensor.
func (SculkSensor) BlockActorID() string { return "SculkSensor" }
