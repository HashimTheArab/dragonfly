package world

import "github.com/df-mc/dragonfly/server/event"

// Context is the owner-scoped context passed to world callbacks and scheduled
// world work. It proves that the callback is already running on the world's
// owner. Use ctx.Tx() for world operations rather than scheduling another
// transaction.
type Context struct {
	*event.Context[*Tx]
}

func newContext(tx *Tx) *Context {
	return &Context{Context: event.C(tx)}
}

// Tx returns the transaction backing the context. The returned transaction is
// only valid for the duration of the callback.
func (ctx *Context) Tx() *Tx { return ctx.Val() }

// Defer schedules f to run after the current owner callback completes.
func (ctx *Context) Defer(f func(ctx *Context)) *Task {
	return ctx.Tx().deferTask(func(ctx *Context) error {
		f(ctx)
		return nil
	})
}
