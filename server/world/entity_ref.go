package world

import (
	"context"
	"fmt"
	"time"
)

// EntityRef is a typed stable reference to an EntityHandle. The entity value T
// is only exposed inside scheduled owner callbacks, where it is safe to use.
type EntityRef[T Entity] struct {
	h *EntityHandle
}

// NewEntityRef creates a typed reference from an EntityHandle.
func NewEntityRef[T Entity](h *EntityHandle) EntityRef[T] { return EntityRef[T]{h: h} }

// Handle returns the underlying stable entity handle.
func (r EntityRef[T]) Handle() *EntityHandle { return r.h }

// Schedule schedules f on the referenced entity's current owner. If the entity
// despawns or closes before the task runs, the returned Task records an error.
func (r EntityRef[T]) Schedule(f func(ctx *Context, e T)) *Task {
	if r.h == nil {
		return newFinishedTask(ErrEntityClosed)
	}
	return r.h.schedule(func(ctx *Context, e Entity) error {
		v, ok := e.(T)
		if !ok {
			return fmt.Errorf("%w: got %T", ErrEntityType, e)
		}
		f(ctx, v)
		return nil
	})
}

// ScheduleAfter schedules f on the referenced entity's owner after delay.
func (r EntityRef[T]) ScheduleAfter(delay time.Duration, f func(ctx *Context, e T)) *Task {
	if r.h == nil {
		return newFinishedTask(ErrEntityClosed)
	}
	return r.h.scheduleAfter(delay, func(ctx *Context, e Entity) error {
		v, ok := e.(T)
		if !ok {
			return fmt.Errorf("%w: got %T", ErrEntityType, e)
		}
		f(ctx, v)
		return nil
	})
}

// CallRef schedules f on the typed EntityRef's current world owner and waits
// for its typed result. CallRef is intended for advanced off-owner
// request/response paths. Code that already has a *world.Context or typed
// entity callback should use that context directly instead of calling CallRef.
func CallRef[R any, E Entity](ctx context.Context, ref EntityRef[E], f func(ctx *Context, e E) (R, error)) (R, error) {
	var zero R
	return CallEntity(ctx, ref.h, func(ctx *Context, e Entity) (R, error) {
		v, ok := e.(E)
		if !ok {
			return zero, fmt.Errorf("%w: got %T", ErrEntityType, e)
		}
		return f(ctx, v)
	})
}
