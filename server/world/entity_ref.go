package world

import (
	"context"
	"fmt"
	"time"
)

// EntityRef is a stable, typed reference to an entity. The entity value T is
// only handed to scheduled owner callbacks, where it is safe to use.
type EntityRef[T Entity] struct {
	h *EntityHandle
}

// NewEntityRef creates a typed reference from an EntityHandle.
func NewEntityRef[T Entity](h *EntityHandle) EntityRef[T] { return EntityRef[T]{h: h} }

// Handle returns the underlying stable entity handle.
func (r EntityRef[T]) Handle() *EntityHandle { return r.h }

// Do schedules f on the entity's current world owner, like EntityHandle.Do,
// but hands f the entity as T. If the entity is no longer a T when the task
// runs, the task fails with ErrEntityType.
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

// DoAfter schedules f on the entity's world owner after delay, typed like Do.
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

// CallRef runs f with the ref's entity on its current world owner and waits
// for the typed result. Off-owner code only, like Call.
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

// assertEntity converts e to T, returning ErrEntityType if it no longer is one.
func assertEntity[T Entity](e Entity) (T, error) {
	v, ok := e.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("%w: got %T", ErrEntityType, e)
	}
	return v, nil
}
