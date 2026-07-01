package player

import (
	"github.com/df-mc/dragonfly/server/world"
)

// Context is the owner-scoped context passed to player callbacks. It embeds the
// world Context (world ops + cancellation) and adds the Player the callback
// concerns. Valid only during the callback.
type Context struct {
	*world.Context
	p *Player
}

// newContext returns a player Context for p, backed by a fresh event-scoped
// view of the player's current world transaction.
func newContext(p *Player) *Context {
	return &Context{Context: p.tx.Event(), p: p}
}

// Player returns the player for this callback. The returned player is only
// valid for the duration of the callback.
func (ctx *Context) Player() *Player { return ctx.p }

// Defer schedules f after the current callback completes, handing it a
// player.Context. The player is re-resolved from its handle against the fresh
// deferred transaction (the current one is closed by then). If the player has
// left the world, the returned task fails with world.ErrEntityClosed.
func (ctx *Context) Defer(f func(ctx *Context)) *world.Task {
	h := ctx.p.H()
	return ctx.Context.DeferErr(func(wctx *world.Context) error {
		if e, ok := h.Entity(wctx); ok {
			f(newContext(e.(*Player)))
			return nil
		}
		return world.ErrEntityClosed
	})
}
