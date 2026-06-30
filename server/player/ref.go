package player

import (
	"time"

	"github.com/df-mc/dragonfly/server/world"
)

// Ref is a typed stable reference to a player. The *Player value is only
// exposed inside scheduled owner callbacks, where it is safe to use.
type Ref = world.EntityRef[*Player]

// NewRef creates a typed player reference from an entity handle.
func NewRef(h *world.EntityHandle) Ref { return world.NewEntityRef[*Player](h) }

// Do schedules f on the player's current world owner. The receiver may
// be a transaction-scoped Player; the scheduled callback receives the current
// Player value for the owner context in which it runs.
func (p *Player) Do(f func(ctx *world.Context, p *Player)) *world.Task {
	if p == nil {
		return world.NewFinishedTask(world.ErrEntityClosed)
	}
	return NewRef(p.handle).Do(func(ctx *world.Context, current *Player) {
		f(ctx, current)
	})
}

// DoAfter schedules f on the player's current world owner after delay.
func (p *Player) DoAfter(delay time.Duration, f func(ctx *world.Context, p *Player)) *world.Task {
	if p == nil {
		return world.NewFinishedTask(world.ErrEntityClosed)
	}
	return NewRef(p.handle).DoAfter(delay, func(ctx *world.Context, current *Player) {
		f(ctx, current)
	})
}
