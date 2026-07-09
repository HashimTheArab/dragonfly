package world

import "time"

// Do schedules f to run with the EntityHandle's entity on its current world
// owner. Do returns immediately; if the entity is not currently in a world, the
// task waits until it enters one or the entity closes. The entity value passed
// to f is only valid for the duration of f. If the entity is already bound to a
// synchronous World, f runs before Do returns.
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

// schedule runs f on the entity's current world owner via a goroutine that
// waits for the entity to be world-bound and follows it across world changes.
// It deliberately does not fast-path onto the current world's queue — that
// commits to one world and would fail instead of follow the entity on migration.
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
	w := e.trackCloseSchedule(task)
	if !task.pending() {
		return task
	}
	run := func() {
		if w != nil {
			defer w.scheduling.Done()
		}
		e.runScheduled(task, f, w)
	}
	if e.currentWorldSynchronous() {
		run()
	} else {
		go run()
	}
	return task
}

// scheduleAfter is the entity counterpart to World.DoAfter. It runs its own
// timer loop (not World.DoAfter) because the entity may change worlds during
// the delay, so it re-acquires world signals each iteration.
func (e *EntityHandle) scheduleAfter(delay time.Duration, f func(ctx *Context, e Entity) error) *Task {
	if delay <= 0 {
		return e.schedule(f)
	}
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
				e.runScheduled(task, f, nil)
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
	}()
	return task
}

// trackCloseSchedule marks immediate entity work created during the world's
// close transaction so World.close drains it before shutting the queue down.
func (e *EntityHandle) trackCloseSchedule(task *Task) *World {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()
	w := e.w
	if w == nil || w == closeWorld || !w.closed.Load() {
		return nil
	}
	w.scheduleMu.Lock()
	defer w.scheduleMu.Unlock()
	if !w.closeAcceptingEntityTasks.Load() {
		task.failIfPending(ErrWorldClosed)
		return nil
	}
	select {
	case <-w.queueClosing:
		task.failIfPending(ErrWorldClosed)
		return nil
	default:
		w.scheduling.Add(1)
		return w
	}
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

// currentWorldSynchronous reports whether the entity is bound to a
// synchronous World.
func (e *EntityHandle) currentWorldSynchronous() bool {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()
	return e.w != nil && e.w != closeWorld && e.worldReady && e.w.conf.Synchronous
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
func (e *EntityHandle) runScheduled(task *Task, f func(ctx *Context, e Entity) error, allowedCloseWorld *World) {
	run := e.execWorld(func(ctx *Context, ent Entity) {
		if !task.begin() {
			return
		}
		err := executeWithRecovery(func() error { return f(ctx, ent) })
		ctx.runDeferred()
		task.finish(err)
	}, false, task.Done(), allowedCloseWorld)
	if !run || task.pending() {
		err := ErrEntityClosed
		if e.currentWorldClosing() {
			err = ErrWorldClosed
		}
		task.failIfPending(err)
	}
}
