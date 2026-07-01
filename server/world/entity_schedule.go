package world

import "time"

// Do schedules f to run with the EntityHandle's entity on its current world
// owner. Do returns immediately; if the entity is not currently in a world, the
// task waits until it enters one or the entity closes. The entity value passed
// to f is only valid for the duration of f.
func (e *EntityHandle) Do(f func(ctx *Context, e Entity)) *Task {
	return e.schedule(func(ctx *Context, e Entity) error {
		f(ctx, e)
		return nil
	})
}

// DoAfter schedules f to run with the EntityHandle's entity after delay.
func (e *EntityHandle) DoAfter(delay time.Duration, f func(ctx *Context, e Entity)) *Task {
	return e.scheduleAfter(delay, func(ctx *Context, e Entity) error {
		f(ctx, e)
		return nil
	})
}

// schedule enqueues f to run on the entity's current world owner. A goroutine
// waits (via execWorld) for the entity to be world-bound if it is not already,
// and follows the entity if it migrates between worlds before f runs.
//
// A tempting optimisation is to skip the goroutine and queue directly on the
// entity's current world when it is already world-bound. That does not work:
// the transaction is committed to one specific world, so if the entity
// migrates (death→respawn, portal) between scheduling and execution the task
// fails instead of following the entity. Blocking consumers such as
// session.withControllable treat that failure as terminal and would disconnect
// a live player or permanently stop a background loop. Correctness requires the
// migration-following execWorld path, so schedule always uses it.
func (e *EntityHandle) schedule(f func(ctx *Context, e Entity) error) *Task {
	task := newTask()
	if e == nil {
		task.failIfPending(ErrEntityClosed)
		return task
	}
	task.setCancel(func() {
		e.cond.L.Lock()
		e.cond.Broadcast()
		e.cond.L.Unlock()
	})
	go e.runScheduled(task, f)
	return task
}

// scheduleAfter is the entity counterpart to World.DoAfter. It manages its
// own timer loop rather than delegating to DoAfter because entities can move
// between worlds during the delay — the loop re-acquires world signals on
// each iteration via worldChanged. World.DoAfter does not need this because
// worlds don't migrate.
func (e *EntityHandle) scheduleAfter(delay time.Duration, f func(ctx *Context, e Entity) error) *Task {
	task := newTask()
	if e == nil {
		task.failIfPending(ErrEntityClosed)
		return task
	}
	task.setCancel(func() {
		e.cond.L.Lock()
		e.cond.Broadcast()
		e.cond.L.Unlock()
	})
	go func() {
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			for {
				closeStarted, worldChanged := e.currentWorldSignals()
				select {
				case <-timer.C:
					if e.currentWorldClosing() {
						task.failIfPending(ErrWorldClosed)
						return
					}
					e.runScheduled(task, f)
					return
				case <-task.Done():
					return
				case <-closeStarted:
					if e.currentWorldCloseStarted() == closeStarted {
						task.failIfPending(ErrWorldClosed)
						return
					}
				case <-worldChanged:
				case <-e.closed:
					task.failIfPending(ErrEntityClosed)
					return
				}
			}
		}
		e.runScheduled(task, f)
	}()
	return task
}

// currentWorldSignals returns the close and world-change channels under lock.
func (e *EntityHandle) currentWorldSignals() (<-chan struct{}, <-chan struct{}) {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()
	if e.w == nil || e.w == closeWorld {
		return nil, e.worldChanged
	}
	return e.w.closeStarted, e.worldChanged
}

// currentWorldCloseStarted returns the closeStarted channel of the entity's
// current world, or nil if the entity is not in a world.
func (e *EntityHandle) currentWorldCloseStarted() <-chan struct{} {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()
	if e.w == nil || e.w == closeWorld {
		return nil
	}
	return e.w.closeStarted
}

// currentWorldClosing checks under lock whether the entity's current world
// has started closing.
func (e *EntityHandle) currentWorldClosing() bool {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()
	if e.w == nil || e.w == closeWorld {
		return false
	}
	select {
	case <-e.w.closeStarted:
		return true
	default:
		return false
	}
}

// runScheduled executes the scheduled entity callback via execWorld, using
// the same completion model as scheduledTransaction: run -> drain deferred ->
// finish task.
func (e *EntityHandle) runScheduled(task *Task, f func(ctx *Context, e Entity) error) {
	run := e.execWorld(func(ctx *Context, ent Entity) {
		if ctx.World().closed.Load() {
			task.failIfPending(ErrWorldClosed)
			return
		}
		if !task.begin() {
			return
		}
		err := executeWithRecovery(func() error { return f(ctx, ent) })
		ctx.runDeferred()
		task.finish(err)
	}, false, task.Done())
	if !run || task.pending() {
		task.failIfPending(ErrEntityClosed)
	}
}
