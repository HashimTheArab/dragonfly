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

func (e *EntityHandle) currentWorldSignals() (<-chan struct{}, <-chan struct{}) {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()
	if e.w == nil || e.w == closeWorld {
		return nil, e.worldChanged
	}
	return e.w.closeStarted, e.worldChanged
}

func (e *EntityHandle) currentWorldCloseStarted() <-chan struct{} {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()
	if e.w == nil || e.w == closeWorld {
		return nil
	}
	return e.w.closeStarted
}

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

func (e *EntityHandle) runScheduled(task *Task, f func(ctx *Context, e Entity) error) {
	run := e.execWorld(func(tx *Tx, ent Entity) {
		if tx.World().closed.Load() {
			task.failIfPending(ErrWorldClosed)
			return
		}
		if !task.begin() {
			return
		}
		ctx := newContext(tx)
		err := executeWithRecovery(func() error { return f(ctx, ent) })
		tx.deferTask(func(*Context) error {
			task.finish(err)
			return nil
		})
	}, false, task.Done())
	if !run || task.pending() {
		task.failIfPending(ErrEntityClosed)
	}
}
