package world

// Structure represents a structure which may be placed in the world. It has fixed dimensions.
type Structure interface {
	// Dimensions returns the dimensions of the structure. It returns an int array with the width, height and
	// length respectively.
	Dimensions() [3]int
	// At returns the block at a specific location in the structure. When the structure is placed in the
	// world, this method is called for every location within the dimensions of the structure. Additionally,
	// At can return a Liquid to be placed in the same place as the block.
	// At can return nil to not place any block at the position. Returning Air will set any block at that
	// position to air, but returning nil will not do anything.
	// In addition to the coordinates, At will have a function passed that may be used to get a block at a
	// specific position. In scope of At(), structures should use this over World.Block(), due to the way
	// chunks are locked.
	At(x, y, z int, blockAt func(x, y, z int) Block) (Block, Liquid)
}

// UniformStructure may be implemented by Structures that place the same block
// and liquid at every position. BuildStructure uses this information to take
// the same fast path as FillVolume. If ok is false, BuildStructure falls back
// to At or RuntimeIDAt.
type UniformStructure interface {
	Structure
	Uniform() (block Block, liquid Liquid, ok bool)
}

// RuntimeIDPlacement describes the blocks to place at a location in a
// RuntimeIDStructure.
type RuntimeIDPlacement struct {
	// BlockRuntimeID is the foreground block runtime ID to place when
	// PlaceBlock is true.
	BlockRuntimeID uint32
	// Block is the foreground block value to store as block entity data if
	// BlockRuntimeID belongs to an NBT block.
	Block Block
	// PlaceBlock specifies if the foreground block should be written.
	PlaceBlock bool
	// LiquidRuntimeID is the liquid runtime ID to place on layer 1 when
	// PlaceLiquid is true.
	LiquidRuntimeID uint32
	// PlaceLiquid specifies if a liquid should be written. If false, layer 1
	// is cleared if present, matching Structure.At returning a nil Liquid.
	PlaceLiquid bool
}

// RuntimeIDStructure may be implemented by Structures that already store block
// runtime IDs directly, such as structures backed by schematic or clipboard
// data. It avoids converting a runtime ID to a Block only for BuildStructure to
// hash that Block back into a runtime ID. Structures that naturally produce
// Block values should generally implement only Structure and rely on
// BuildStructure's internal runtime ID cache.
//
// Structures implementing RuntimeIDStructure are dispatched through RuntimeIDAt
// exclusively in BuildStructure. At is unused in that path, but is still
// required by the embedded Structure interface.
//
// Runtime IDs returned by this method must be valid runtime IDs, such as IDs
// produced by BlockRuntimeID. Returning an invalid runtime ID panics.
type RuntimeIDStructure interface {
	Structure
	RuntimeIDAt(x, y, z int, blockAt func(x, y, z int) Block) RuntimeIDPlacement
}
