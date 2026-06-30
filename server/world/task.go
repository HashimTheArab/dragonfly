package world

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrWorldClosed is returned by scheduled tasks that could not run because
	// the target world is closed or closing.
	ErrWorldClosed = errors.New("world: world closed")
	// ErrEntityClosed is returned by scheduled entity tasks when the entity is
	// closed before the task can run.
	ErrEntityClosed = errors.New("world: entity closed")
	// ErrTaskCancelled is returned by tasks cancelled before they started.
	ErrTaskCancelled = errors.New("world: scheduled task cancelled")
	// ErrTaskPanicked wraps a panic recovered from a scheduled callback.
	ErrTaskPanicked = errors.New("world: scheduled task panicked")
	// ErrEntityType is returned by typed entity refs when the entity no longer
	// has the expected dynamic type when the scheduled task runs.
	ErrEntityType = errors.New("world: unexpected entity type")
)

// PanicError wraps a recovered panic value, preserving it for re-panicking
// with the original value while supporting errors.Is(err, ErrTaskPanicked).
type PanicError struct {
	Value any
}

func (e *PanicError) Error() string { return fmt.Sprintf("world: scheduled task panicked: %v", e.Value) }
func (e *PanicError) Unwrap() error { return ErrTaskPanicked }

const (
	taskPending int32 = iota
	taskRunning
	taskDone
	taskCancelled
)

// Task is a handle to work scheduled onto a world or entity owner. The default
// use of a Task is fire-and-forget; Done, Err and Wait exist for tests,
// shutdown paths and other code that is known not to be running on the same
// owner context.
type Task struct {
	done  chan struct{}
	state atomic.Int32

	errMu sync.Mutex
	err   error

	cancelMu sync.Mutex
	onCancel func()
}

func newTask() *Task {
	return &Task{done: make(chan struct{})}
}

func newFinishedTask(err error) *Task {
	t := newTask()
	t.failIfPending(err)
	return t
}

// Done returns a channel that is closed when the task has finished, failed, or
// been cancelled.
func (t *Task) Done() <-chan struct{} {
	if t == nil {
		return closedTaskDone()
	}
	return t.done
}

var closedDone = func() <-chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}()

func closedTaskDone() <-chan struct{} { return closedDone }

// Err returns the task error once Done is closed. It returns nil if the task
// completed successfully or has not completed yet.
func (t *Task) Err() error {
	if t == nil {
		return ErrTaskCancelled
	}
	select {
	case <-t.done:
		t.errMu.Lock()
		defer t.errMu.Unlock()
		return t.err
	default:
		return nil
	}
}

// Wait waits for the task to finish or for ctx to be cancelled. Wait should
// not be called from a callback that is already running on the same world or
// entity owner; doing so would recreate the blocking pattern Do is meant
// to avoid.
func (t *Task) Wait(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if t == nil {
		return ErrTaskCancelled
	}
	select {
	case <-t.done:
		return t.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Cancel attempts to cancel the task before it starts. It returns true if the
// task was still pending and will not run.
func (t *Task) Cancel() bool {
	if t == nil || !t.state.CompareAndSwap(taskPending, taskCancelled) {
		return false
	}
	t.setErr(ErrTaskCancelled)
	close(t.done)
	t.runCancel()
	return true
}

func (t *Task) begin() bool {
	return t != nil && t.state.CompareAndSwap(taskPending, taskRunning)
}

func (t *Task) failIfPending(err error) bool {
	if t == nil || !t.state.CompareAndSwap(taskPending, taskRunning) {
		return false
	}
	t.finish(err)
	return true
}

func (t *Task) finish(err error) {
	t.setErr(err)
	t.state.Store(taskDone)
	close(t.done)
}

func (t *Task) setErr(err error) {
	t.errMu.Lock()
	t.err = err
	t.errMu.Unlock()
}

func (t *Task) pending() bool {
	return t != nil && t.state.Load() == taskPending
}

func (t *Task) setCancel(f func()) {
	if t == nil || f == nil {
		return
	}
	t.cancelMu.Lock()
	cancelled := t.state.Load() == taskCancelled
	if !cancelled {
		t.onCancel = f
	}
	t.cancelMu.Unlock()
	if cancelled {
		f()
	}
}

func (t *Task) runCancel() {
	t.cancelMu.Lock()
	f := t.onCancel
	t.cancelMu.Unlock()
	if f != nil {
		f()
	}
}

// Do schedules f to run on the world's owner context. Do does not wait for f
// to run and is safe to use from goroutines outside the world owner. Scheduled
// work runs FIFO with other world transactions once queued. If the owner queue
// is saturated, Do still returns without blocking the caller and queues the
// task from a helper goroutine.
func (w *World) Do(f func(ctx *Context)) *Task {
	return w.scheduleTask(newTask(), func(ctx *Context) error {
		f(ctx)
		return nil
	})
}

// DoAfter schedules f to run on the world's owner context after delay.
// If the task is cancelled before delay elapses, f is not queued.
func (w *World) DoAfter(delay time.Duration, f func(ctx *Context)) *Task {
	t := newTask()
	if delay <= 0 {
		return w.scheduleTask(t, func(ctx *Context) error {
			f(ctx)
			return nil
		})
	}
	if w == nil || w.queue == nil || w.closed.Load() {
		t.failIfPending(ErrWorldClosed)
		return t
	}
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
			w.scheduleTask(t, func(ctx *Context) error {
				f(ctx)
				return nil
			})
		case <-t.Done():
		case <-w.closeStarted:
			t.failIfPending(ErrWorldClosed)
		case <-w.closing:
			t.failIfPending(ErrWorldClosed)
		case <-w.queueClosing:
			t.failIfPending(ErrWorldClosed)
		}
	}()
	return t
}

// Call schedules f on the world's owner context and waits for its typed result.
// Call is intended for off-owner request/response paths such as tests,
// startup/shutdown, and background goroutines. Code that already has a
// *world.Context should call the context methods directly instead of Call.
func Call[T any](ctx context.Context, w *World, f func(ctx *Context) (T, error)) (T, error) {
	var zero T
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	default:
	}
	var result T
	task := w.scheduleTask(newTask(), func(wctx *Context) error {
		var err error
		result, err = f(wctx)
		return err
	})
	select {
	case <-task.Done():
		if err := task.Err(); err != nil {
			return zero, err
		}
		return result, nil
	case <-ctx.Done():
		task.Cancel()
		return zero, ctx.Err()
	}
}

// CallEntity schedules f on the EntityHandle's current world owner and waits
// for its typed result. CallEntity is intended for advanced off-owner
// request/response paths. Code that already has a *world.Context or entity
// callback should use that context directly instead of calling CallEntity.
func CallEntity[R any](ctx context.Context, h *EntityHandle, f func(ctx *Context, e Entity) (R, error)) (R, error) {
	var zero R
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	default:
	}
	if h == nil {
		return zero, ErrEntityClosed
	}
	var result R
	task := h.schedule(func(wctx *Context, e Entity) error {
		var err error
		result, err = f(wctx, e)
		return err
	})
	select {
	case <-task.Done():
		if err := task.Err(); err != nil {
			return zero, err
		}
		return result, nil
	case <-ctx.Done():
		task.Cancel()
		return zero, ctx.Err()
	}
}

func (w *World) scheduleTask(task *Task, f func(ctx *Context) error) *Task {
	if task == nil {
		task = newTask()
	}
	if w == nil || w.queue == nil || w.closed.Load() {
		task.failIfPending(ErrWorldClosed)
		return task
	}
	if !task.pending() {
		return task
	}
	st := scheduledTransaction{task: task, f: f}
	w.scheduleMu.Lock()
	defer w.scheduleMu.Unlock()
	if w.closed.Load() {
		task.failIfPending(ErrWorldClosed)
		return task
	}
	select {
	case <-w.closing:
		task.failIfPending(ErrWorldClosed)
	case <-w.queueClosing:
		task.failIfPending(ErrWorldClosed)
	case w.queue <- st:
	default:
		w.scheduling.Add(1)
		go w.queueScheduled(st)
	}
	return task
}

func (w *World) queueScheduled(st scheduledTransaction) {
	defer w.scheduling.Done()
	if w.closed.Load() {
		st.task.failIfPending(ErrWorldClosed)
		return
	}
	select {
	case <-w.closing:
		st.task.failIfPending(ErrWorldClosed)
	case <-w.queueClosing:
		st.task.failIfPending(ErrWorldClosed)
	case <-st.task.Done():
	case w.queue <- st:
	}
}

type scheduledTransaction struct {
	task *Task
	f    func(ctx *Context) error
}

func (st scheduledTransaction) Run(w *World) {
	if !st.task.begin() {
		return
	}
	tx := &Tx{w: w}
	ctx := newContext(tx)
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = &PanicError{Value: r}
		}
		tx.close()
		tx.runDeferred()
		st.task.finish(err)
	}()
	err = st.f(ctx)
}
