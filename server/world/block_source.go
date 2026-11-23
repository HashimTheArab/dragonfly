package world

import "github.com/df-mc/dragonfly/server/block/cube"

// BlockSource represents a source for obtaining blocks.
type BlockSource interface {
	// Block returns the block at the given position in the block source.
	Block(cube.Pos) Block
}

// LiquidSource is a BlockSource that can also resolve liquids.
type LiquidSource interface {
	BlockSource
	// Liquid returns the liquid at the given position, if any.
	Liquid(cube.Pos) (Liquid, bool)
}

// StateDeriver is implemented by Blocks that derive part of their state from the blocks around them,
// such as the connections of a fence. World uses it to recalculate that state for chunks saved before
// it existed, which hold a default in its place.
type StateDeriver interface {
	Block
	// DeriveState returns the Block with its derived state recalculated against src.
	DeriveState(pos cube.Pos, src BlockSource) Block
}

// MatchesDerivedState reports whether actualID is the runtime ID of wanted at
// pos once wanted derives its state against src. Derived state, such as a
// stair's corner, follows the blocks around pos rather than wanted, so a
// caller comparing a built or predicted block with a wanted one compares it
// here.
func MatchesDerivedState(registry BlockRegistry, src BlockSource, pos cube.Pos, wanted Block, actualID uint32) bool {
	if deriver, ok := wanted.(StateDeriver); ok {
		wanted = deriver.DeriveState(pos, src)
	}
	name, properties := wanted.EncodeBlock()
	wantedID, ok := registry.StateToRuntimeID(name, properties)
	return ok && wantedID == actualID
}

// worldSource is a wrapper around a world transaction that implements BlockSource.
type worldSource struct{ tx *Tx }

func (w worldSource) Block(pos cube.Pos) Block { return w.tx.block(pos) }
