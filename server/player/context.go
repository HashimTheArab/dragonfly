package player

import (
	"github.com/df-mc/dragonfly/server/event"
	"github.com/df-mc/dragonfly/server/world"
)

// Context is the owner-scoped context passed to player callbacks. It proves
// that the callback is already running on the player's world owner. Use
// ctx.Tx() for world operations rather than scheduling another transaction.
type Context struct {
	*event.Context[*Player]
}

// newContext wraps a player in a cancellable owner context.
func newContext(p *Player) *Context {
	return &Context{Context: event.C(p)}
}

// Player returns the player for this callback. The returned player is only
// valid for the duration of the callback.
func (ctx *Context) Player() *Player { return ctx.Val() }

// Tx returns the transaction backing the player callback. The returned
// transaction is only valid for the duration of the callback.
func (ctx *Context) Tx() *world.Tx { return ctx.Player().tx }

// Defer schedules f to run after the current owner callback completes. The
// deferred callback receives a player.Context so the player remains accessible.
//
// The current *Player is bound to the current transaction, which is closed by
// the time the deferred callback runs. The player is therefore re-resolved from
// the stable entity handle against the fresh transaction; if the player has
// left the world by then, f is not run.
func (ctx *Context) Defer(f func(ctx *Context)) *world.Task {
	h := ctx.Player().H()
	return ctx.Tx().Context().Defer(func(wctx *world.Context) {
		if e, ok := h.Entity(wctx.Tx()); ok {
			f(newContext(e.(*Player)))
		}
	})
}
