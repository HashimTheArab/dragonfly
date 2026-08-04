package world

import "github.com/df-mc/dragonfly/server/block/cube"

// BlockSource represents a source for obtaining blocks.
type BlockSource interface {
	// Block returns the block at the given position in the block source.
	Block(cube.Pos) Block
}

// LiquidSource is a BlockSource that can also resolve liquids, which not every source can:
// one standing in for a hypothetical world may only know about blocks.
type LiquidSource interface {
	BlockSource
	// Liquid returns the liquid at the given position, if there is one.
	Liquid(cube.Pos) (Liquid, bool)
}

// worldSource is a wrapper around a world transaction that implements BlockSource.
type worldSource struct{ tx *Tx }

func (w worldSource) Block(pos cube.Pos) Block { return w.tx.block(pos) }
