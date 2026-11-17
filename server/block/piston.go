package block

// Piston supplies block identity and actor metadata. Gameplay behaviour and
// the block-actor NBT codec are not implemented.
type Piston struct{}

// EncodeBlock returns the block identifier without selecting a gameplay state.
func (Piston) EncodeBlock() (string, map[string]any) { return "minecraft:piston", nil }

// BlockActorID returns the Bedrock NBT identifier for Piston.
func (Piston) BlockActorID() string { return "PistonArm" }

// BlockActorAliases returns the sticky piston, which shares the actor.
func (Piston) BlockActorAliases() []string { return []string{"minecraft:sticky_piston"} }
