package block

// HangingSign supplies block identity and actor metadata for every wood
// variant of the hanging sign. Gameplay behaviour and the block-actor NBT codec
// are not implemented.
type HangingSign struct{}

// EncodeBlock returns the oak hanging sign identifier without selecting a
// gameplay state.
func (HangingSign) EncodeBlock() (string, map[string]any) {
	return "minecraft:oak_hanging_sign", nil
}

// BlockActorID returns the Bedrock NBT identifier shared by hanging signs.
func (HangingSign) BlockActorID() string { return "HangingSign" }

// BlockActorAliases returns the hanging sign of every wood from the shared
// vanilla catalog.
func (HangingSign) BlockActorAliases() []string { return catalogNames("_hanging_sign") }
