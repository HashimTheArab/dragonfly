package block

// Beehive supplies block identity and actor metadata. Gameplay behaviour and
// the block-actor NBT codec are not implemented.
type Beehive struct{}

// EncodeBlock returns the block identifier without selecting a gameplay state.
func (Beehive) EncodeBlock() (string, map[string]any) { return "minecraft:beehive", nil }

// BlockActorID returns the Bedrock NBT identifier for Beehive.
func (Beehive) BlockActorID() string { return "Beehive" }

// BlockActorAliases returns the bee nest, which shares the beehive's actor.
func (Beehive) BlockActorAliases() []string { return []string{"minecraft:bee_nest"} }
