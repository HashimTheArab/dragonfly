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

// Do schedules f on the referenced entity's current owner. If the entity
// despawns or closes before the task runs, the returned Task records an error.
func (r EntityRef[T]) Do(f func(ctx *Context, e T)) *Task {
	if r.h == nil {
		return NewFinishedTask(ErrEntityClosed)
	}
	return r.h.schedule(func(ctx *Context, e Entity) error {
		v, err := assertEntity[T](e)
		if err != nil {
			return err
		}
		f(ctx, v)
		return nil
	})
}

// DoAfter schedules f on the referenced entity's owner after delay.
func (r EntityRef[T]) DoAfter(delay time.Duration, f func(ctx *Context, e T)) *Task {
	if r.h == nil {
		return NewFinishedTask(ErrEntityClosed)
	}
	return r.h.scheduleAfter(delay, func(ctx *Context, e Entity) error {
		v, err := assertEntity[T](e)
		if err != nil {
			return err
		}
		f(ctx, v)
		return nil
	})
}

// CallRef schedules f on the ref's current world owner and waits for its typed
// result. For off-owner code only; if you already have a *world.Context, use it
// directly. Like Call, never call it from the owner goroutine (see Call).
func CallRef[R any, E Entity](ctx context.Context, ref EntityRef[E], f func(ctx *Context, e E) (R, error)) (R, error) {
	var zero R
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	default:
	}
	if ref.h == nil {
		return zero, ErrEntityClosed
	}
	var result R
	task := ref.h.schedule(func(wctx *Context, e Entity) error {
		v, err := assertEntity[E](e)
		if err != nil {
			return err
		}
		var callErr error
		result, callErr = f(wctx, v)
		return callErr
	})
	return awaitTask(ctx, task, &result)
}

// assertEntity asserts that e is of type T, returning ErrEntityType if not.
func assertEntity[T Entity](e Entity) (T, error) {
	v, ok := e.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("%w: got %T", ErrEntityType, e)
	}
	return v, nil
}
