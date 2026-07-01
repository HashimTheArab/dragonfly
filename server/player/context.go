package player

import (
	"github.com/df-mc/dragonfly/server/world"
)

// Context is the owner-scoped context passed to player callbacks. It embeds the
// world owner Context, so world operations (SetBlock, AddEntity, ...) and event
// cancellation are available directly, and additionally exposes the Player the
// callback concerns. A Context is valid only for the duration of the callback.
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

// Defer schedules f to run after the current owner callback completes. The
// deferred callback receives a player.Context so the player remains accessible.
//
// The current *Player is bound to the current transaction, which is closed by
// the time the deferred callback runs. The player is therefore re-resolved from
// the stable entity handle against the fresh transaction; if the player has
// left the world by then, f is not run.
func (ctx *Context) Defer(f func(ctx *Context)) *world.Task {
	h := ctx.p.H()
	return ctx.Context.Defer(func(wctx *world.Context) {
		if e, ok := h.Entity(wctx); ok {
			f(newContext(e.(*Player)))
		}
	})
}
