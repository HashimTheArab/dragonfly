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

// Error implements the error interface.
func (e *PanicError) Error() string {
	return fmt.Sprintf("world: scheduled task panicked: %v", e.Value)
}

// Unwrap returns ErrTaskPanicked so errors.Is works.
func (e *PanicError) Unwrap() error { return ErrTaskPanicked }

// executeWithRecovery runs f, recovering any panic into a *PanicError.
func executeWithRecovery(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &PanicError{Value: r}
		}
	}()
	return f()
}

// awaitTask waits for a task to complete or the context to cancel, returning
// the result stored by the callback through the result pointer.
func awaitTask[T any](ctx context.Context, task *Task, result *T) (T, error) {
	var zero T
	select {
	case <-task.Done():
		if err := task.Err(); err != nil {
			return zero, err
		}
		return *result, nil
	case <-ctx.Done():
		task.Cancel()
		return zero, ctx.Err()
	}
}

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

// newTask returns a pending Task with an open done channel.
func newTask() *Task {
	return &Task{done: make(chan struct{})}
}

// NewFinishedTask returns a Task that is already completed with the given
// error.
func NewFinishedTask(err error) *Task {
	t := newTask()
	t.failIfPending(err)
	return t
}

// closedDone is the Done channel returned for nil tasks.
var closedDone = func() <-chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}()

// Done returns a channel that is closed when the task has finished, failed, or
// been cancelled.
func (t *Task) Done() <-chan struct{} {
	if t == nil {
		return closedDone
	}
	return t.done
}

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

// OnDone spawns a goroutine that calls f with the task's error once it
// completes. If the task is nil, f is called immediately with
// ErrTaskCancelled.
func (t *Task) OnDone(f func(err error)) {
	if t == nil {
		f(ErrTaskCancelled)
		return
	}
	go func() {
		<-t.done
		f(t.Err())
	}()
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

// begin transitions the task from pending to running. Returns false if the
// task was already started or cancelled.
func (t *Task) begin() bool {
	return t != nil && t.state.CompareAndSwap(taskPending, taskRunning)
}

// failIfPending transitions the task from pending to done with the given
// error. Returns false if the task was no longer pending.
func (t *Task) failIfPending(err error) bool {
	if t == nil || !t.state.CompareAndSwap(taskPending, taskRunning) {
		return false
	}
	t.finish(err)
	return true
}

// finish completes the task, storing err and closing the done channel.
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

// setCancel registers a function to run if the task is cancelled. If the
// task is already cancelled when setCancel is called, f runs immediately.
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

// runCancel invokes the registered cancel function, if any.
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
// task from a helper goroutine. On a synchronous World, f runs before Do
// returns.
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

// Call schedules f on w's owner and waits for its typed result. It is for
// off-owner code (tests, startup, background goroutines); if you already have a
// *world.Context, use it directly. Never call it from the owner goroutine (a
// scheduled callback or Handler event) — it deadlocks waiting on that owner.
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
	return awaitTask(ctx, task, &result)
}

// CallEntity schedules f on the EntityHandle's current world owner and waits
// for its typed result. It is shorthand for CallRef with Entity as the type.
// Like Call, it must not be used from the owner goroutine (see Call).
func CallEntity[R any](ctx context.Context, h *EntityHandle, f func(ctx *Context, e Entity) (R, error)) (R, error) {
	return CallRef(ctx, NewEntityRef[Entity](h), f)
}

// scheduleTask enqueues a scheduledTransaction on the world's owner queue.
// If the queue is full, a helper goroutine is spawned to avoid blocking.
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
	if w.closed.Load() {
		w.scheduleMu.Unlock()
		task.failIfPending(ErrWorldClosed)
		return task
	}
	if w.conf.Synchronous {
		w.scheduleMu.Unlock()
		st.Run(w)
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
	w.scheduleMu.Unlock()
	return task
}

// queueScheduled is a helper goroutine that retries enqueuing a transaction
// when the world queue was full at the time of scheduling.
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

// scheduledTransaction is a task-aware transaction queued via Do, DoAfter,
// or Context.Defer. It creates its own Context, runs the callback with panic
// recovery, drains deferred work, then finishes the task.
type scheduledTransaction struct {
	task *Task
	f    func(ctx *Context) error
}

// Run executes the scheduled callback on the world goroutine.
func (st scheduledTransaction) Run(w *World) {
	if !st.task.begin() {
		return
	}
	ctx := newContext(w)
	err := executeWithRecovery(func() error { return st.f(ctx) })
	ctx.close()
	ctx.runDeferred()
	st.task.finish(err)
}
