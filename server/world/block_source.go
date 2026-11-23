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

// AmbientStater is implemented by Blocks with state that changes on its own
// after they are placed, such as the growth of a crop or the moisture of
// farmland, rather than through how they were placed.
type AmbientStater interface {
	Block
	// WithAmbientState returns the Block with the ambient state of other copied
	// in. It returns false when other is a different kind of block.
	WithAmbientState(other Block) (Block, bool)
}

// MatchesBuiltState reports whether actual, built at pos in src, is wanted.
// Neither the state wanted derives from its neighbours, such as a stair's
// corner, nor its ambient state, such as a crop's growth, is compared as
// wanted holds it: placing a block chooses neither. wanted takes actual's
// ambient state and then derives against src before the runtime IDs compare.
func MatchesBuiltState(registry BlockRegistry, src BlockSource, pos cube.Pos, wanted, actual Block) bool {
	if registry == nil || wanted == nil || actual == nil {
		return false
	}
	if ambient, ok := wanted.(AmbientStater); ok {
		if b, ok := ambient.WithAmbientState(actual); ok {
			wanted = b
		}
	}
	if deriver, ok := wanted.(StateDeriver); ok {
		if src == nil {
			return false
		}
		wanted = deriver.DeriveState(pos, src)
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
