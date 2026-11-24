package world

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
)

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

// MatchesDerivedState reports whether actual, built or predicted at pos, is
// wanted, either exactly or once wanted derives its state against src.
// Derived state, such as a stair's corner, follows the blocks around pos
// rather than wanted, so a caller comparing a built or predicted block with a
// wanted one compares it here. An exact match needs no terrain: actual is the
// state asked for. Blocks are compared by hash, and only encoded when one has
// none.
func MatchesDerivedState(registry BlockRegistry, src BlockSource, pos cube.Pos, wanted, actual Block) bool {
	if registry == nil || wanted == nil || actual == nil {
		return false
	}
	deriver, derives := wanted.(StateDeriver)
	wantedBase, wantedHash := wanted.Hash()
	actualBase, actualHash := actual.Hash()
	hashed := wantedHash != math.MaxUint64 && actualHash != math.MaxUint64
	if hashed {
		// Deriving never changes the kind of block, and a block that derives
		// nothing matches only its own state.
		if wantedBase != actualBase {
			return false
		}
		if wantedHash == actualHash {
			return true
		}
		if !derives {
			return false
		}
	}
	if derives {
		if src == nil {
			return false
		}
		wanted = deriver.DeriveState(pos, src)
		if base, hash := wanted.Hash(); hashed && hash != math.MaxUint64 {
			return base == actualBase && hash == actualHash
		}
	}
	wantedName, wantedProperties := wanted.EncodeBlock()
	actualName, actualProperties := actual.EncodeBlock()
	wantedID, wantedKnown := registry.StateToRuntimeID(wantedName, wantedProperties)
	actualID, actualKnown := registry.StateToRuntimeID(actualName, actualProperties)
	return wantedKnown && actualKnown && wantedID == actualID
}

// worldSource is a wrapper around a world transaction that implements BlockSource.
type worldSource struct{ tx *Tx }

func (w worldSource) Block(pos cube.Pos) Block { return w.tx.block(pos) }
