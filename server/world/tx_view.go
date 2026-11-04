package world

import "github.com/df-mc/dragonfly/server/block/cube"

// BlockTransactionView is a detached block view used to run block behaviour
// without loading chunks in a World. Reads and writes made through the
// transaction are delegated to the view. Non-block side effects such as
// particles, sounds and scheduled updates are discarded. Any other world
// operation terminates the calculation and makes its result incomplete.
//
// Implementations must make writes immediately visible through Block and
// Liquid so that multi-block behaviour observes its earlier changes.
type BlockTransactionView interface {
	LiquidSource
	BlockLoaded(pos cube.Pos) (Block, bool)
	Range() cube.Range
	Dimension() Dimension
	SetBlock(pos cube.Pos, b Block, opts *SetOpts)
	SetLiquid(pos cube.Pos, liquid Liquid)
}

// RunBlockTransaction runs f with a transaction backed by view. It is intended
// for detached block-state calculations such as placement prediction, where
// invoking the block's real behaviour is preferable to copying it into a
// second implementation.
//
// The transaction must not escape f. RunBlockTransaction is synchronous and
// does not retain view after f returns. It returns false when the callback used
// world state that the detached view cannot represent.
func RunBlockTransaction(view BlockTransactionView, f func(tx *Tx)) (complete bool) {
	if view == nil || f == nil {
		return false
	}
	tx := newTx(nil)
	tx.view = view
	defer tx.close()
	defer func() {
		if recovered := recover(); recovered != nil {
			if _, ok := recovered.(detachedTransactionUnsupported); !ok {
				panic(recovered)
			}
			complete = false
		}
	}()
	f(tx)
	return !tx.viewIncomplete
}

type detachedTransactionUnsupported struct{}

func (tx *Tx) rejectDetached() {
	if tx.view != nil {
		tx.viewIncomplete = true
		panic(detachedTransactionUnsupported{})
	}
}
