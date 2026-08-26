package world

import "github.com/df-mc/dragonfly/server/block/cube"

// BlockTransactionView is a detached block view used to run block behaviour
// without loading chunks in a World. Reads and writes made through the
// transaction are delegated to the view. Non-block side effects such as
// particles, sounds and scheduled updates are discarded.
//
// Implementations must make writes immediately visible through Block and
// Liquid so that multi-block behaviour observes its earlier changes.
type BlockTransactionView interface {
	LiquidSource
	Range() cube.Range
	Dimension() Dimension
	SetBlock(pos cube.Pos, b Block)
	SetLiquid(pos cube.Pos, liquid Liquid)
}

// RunBlockTransaction runs f with a transaction backed by view. It is intended
// for detached block-state calculations such as placement prediction, where
// invoking the block's real behaviour is preferable to copying it into a
// second implementation.
//
// The transaction and its World must not escape f. RunBlockTransaction is
// synchronous and does not retain view after f returns.
func RunBlockTransaction(view BlockTransactionView, f func(tx *Tx)) {
	if view == nil || f == nil {
		return
	}
	dim := view.Dimension()
	if dim == nil {
		dim = Overworld
	}
	settings := defaultSettings()
	w := &World{
		conf:     Config{Dim: dim, ReadOnly: true, Synchronous: true},
		ra:       view.Range(),
		set:      settings,
		redstone: newRedstoneEngine(settings.CurrentTick),
	}
	tx := newTx(w)
	tx.view = view
	defer tx.close()
	f(tx)
}
