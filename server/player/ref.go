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

// Schedule schedules f on the player's current world owner. The receiver may
// be a transaction-scoped Player; the scheduled callback receives the current
// Player value for the owner context in which it runs.
func (p *Player) Schedule(f func(p *Player, ctx *world.Context)) *world.Task {
	if p == nil {
		return world.NewEntityRef[*Player](nil).Schedule(func(*world.Context, *Player) {})
	}
	return NewRef(p.handle).Schedule(func(ctx *world.Context, current *Player) {
		f(current, ctx)
	})
}

// ScheduleAfter schedules f on the player's current world owner after delay.
func (p *Player) ScheduleAfter(delay time.Duration, f func(p *Player, ctx *world.Context)) *world.Task {
	if p == nil {
		return world.NewEntityRef[*Player](nil).Schedule(func(*world.Context, *Player) {})
	}
	return NewRef(p.handle).ScheduleAfter(delay, func(ctx *world.Context, current *Player) {
		f(current, ctx)
	})
}
