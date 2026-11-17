package block

// Shelf supplies block identity and actor metadata for every wood variant of
// the shelf. Gameplay behaviour and the block-actor NBT codec are not
// implemented.
type Shelf struct{}

// EncodeBlock returns the oak shelf identifier without selecting a gameplay
// state.
func (Shelf) EncodeBlock() (string, map[string]any) { return "minecraft:oak_shelf", nil }

// BlockActorID returns the Bedrock NBT identifier shared by shelves.
func (Shelf) BlockActorID() string { return "Shelf" }

// BlockActorAliases returns the shelf of every wood from the shared vanilla
// catalog.
func (Shelf) BlockActorAliases() []string { return catalogNames("_shelf") }
