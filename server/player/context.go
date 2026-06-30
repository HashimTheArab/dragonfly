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

func newContext(p *Player) *Context {
	return &Context{Context: event.C(p)}
}

// Player returns the player for this callback. The returned player is only
// valid for the duration of the callback.
func (ctx *Context) Player() *Player { return ctx.Val() }

// Tx returns the transaction backing the player callback. The returned
// transaction is only valid for the duration of the callback.
func (ctx *Context) Tx() *world.Tx { return ctx.Player().tx }

// Defer schedules f to run after the current owner callback completes.
func (ctx *Context) Defer(f func(ctx *world.Context)) *world.Task {
	return ctx.Tx().Context().Defer(f)
}
