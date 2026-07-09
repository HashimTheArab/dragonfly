package world

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

func TestDoRunsOnWorldContext(t *testing.T) {
	w := New()
	defer w.Close()

	task := w.Do(func(tx *Tx) {
		if tx.w == nil {
			t.Error("scheduled transaction has nil world")
		}
	})
	if err := task.Wait(testContext(t)); err != nil {
		t.Fatalf("scheduled task failed: %v", err)
	}
}

func TestCallReturnsTypedResult(t *testing.T) {
	w := New()
	defer w.Close()

	got, err := Call(testContext(t), w, func(tx *Tx) (int64, error) {
		return tx.CurrentTick(), nil
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if got < 0 {
		t.Fatalf("Call returned negative tick: %v", got)
	}
}

func TestCallDoesNotRunCancelledContext(t *testing.T) {
	w := New()
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var ran atomic.Bool
	_, err := Call(ctx, w, func(tx *Tx) (int, error) {
		ran.Store(true)
		return 1, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if ran.Load() {
		t.Fatal("Call scheduled work after context was cancelled")
	}
}

func TestCallEntityReturnsTypedResult(t *testing.T) {
	w := New()
	defer w.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	got, err := CallEntity(testContext(t), h, func(tx *Tx, e Entity) (mgl64.Vec3, error) {
		if tx.w == nil {
			t.Error("entity call transaction has nil world")
		}
		return e.Position(), nil
	})
	if err != nil {
		t.Fatalf("CallEntity failed: %v", err)
	}
	if got != (mgl64.Vec3{}) {
		t.Fatalf("CallEntity returned unexpected position: %v", got)
	}
}

func TestCallEntityDoesNotRunCancelledContext(t *testing.T) {
	w := New()
	defer w.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var ran atomic.Bool
	_, err := CallEntity(ctx, h, func(tx *Tx, e Entity) (int, error) {
		ran.Store(true)
		return 1, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if ran.Load() {
		t.Fatal("CallEntity scheduled work after context was cancelled")
	}
}

func TestCallRefReturnsTypedResult(t *testing.T) {
	w := New()
	defer w.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	got, err := CallRef(testContext(t), NewEntityRef[taskTestEntity](h), func(tx *Tx, e taskTestEntity) (bool, error) {
		return e.H() == h, nil
	})
	if err != nil {
		t.Fatalf("CallRef failed: %v", err)
	}
	if !got {
		t.Fatal("CallRef did not pass typed entity")
	}
}

func TestCallRefReportsTypeMismatch(t *testing.T) {
	w := New()
	defer w.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	_, err := CallRef(testContext(t), NewEntityRef[markedTaskEntity](h), func(tx *Tx, e markedTaskEntity) (int, error) {
		t.Error("CallRef ran typed callback for mismatched entity")
		return 0, nil
	})
	if !errors.Is(err, ErrEntityType) {
		t.Fatalf("expected ErrEntityType, got %v", err)
	}
}

func TestDoAfterCancel(t *testing.T) {
	w := New()
	defer w.Close()

	ran := make(chan struct{})
	task := w.DoAfter(time.Hour, func(tx *Tx) { close(ran) })
	if !task.Cancel() {
		t.Fatal("expected pending delayed task to cancel")
	}
	if err := task.Wait(testContext(t)); !errors.Is(err, ErrTaskCancelled) {
		t.Fatalf("expected ErrTaskCancelled, got %v", err)
	}
	select {
	case <-ran:
		t.Fatal("cancelled delayed task ran")
	default:
	}
}

func TestDoAfterFailsWhenWorldCloseStarts(t *testing.T) {
	w := New()

	task := w.DoAfter(time.Hour, func(*Tx) {
		t.Error("delayed task ran after world close started")
	})
	errc := make(chan error, 1)
	w.Handle(closeWaitTaskHandler{task: task, errc: errc})

	if err := w.Close(); err != nil {
		t.Fatalf("close world: %v", err)
	}
	if err := <-errc; !errors.Is(err, ErrWorldClosed) {
		t.Fatalf("expected ErrWorldClosed in HandleClose, got %v", err)
	}
}

func TestEntityDoAfterFailsWhenWorldCloseStarts(t *testing.T) {
	w := New()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	task := h.DoAfter(time.Hour, func(*Tx, Entity) {
		t.Error("delayed entity task ran after world close started")
	})
	errc := make(chan error, 1)
	w.Handle(closeWaitTaskHandler{task: task, errc: errc})

	if err := w.Close(); err != nil {
		t.Fatalf("close world: %v", err)
	}
	if err := <-errc; !errors.Is(err, ErrWorldClosed) {
		t.Fatalf("expected ErrWorldClosed in HandleClose, got %v", err)
	}
}

func TestEntityDoAfterFollowsMovedEntity(t *testing.T) {
	w1 := New()
	w2 := New()
	defer w2.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w1.exec(func(tx *Tx) { tx.AddEntity(h) })

	// Remove the entity from w1 before scheduling: whenever the timer fires,
	// the callback can only run once the entity has been added to w2.
	<-w1.exec(func(tx *Tx) {
		e, ok := h.Entity(tx)
		if !ok {
			t.Error("entity missing before move")
			return
		}
		tx.RemoveEntity(e)
	})
	task := h.DoAfter(50*time.Millisecond, func(tx *Tx, _ Entity) {
		if tx.World() != w2 {
			t.Error("delayed entity task did not follow moved entity")
		}
	})
	if err := w1.Close(); err != nil {
		t.Fatalf("close old world: %v", err)
	}
	<-w2.exec(func(tx *Tx) { tx.AddEntity(h) })
	if err := task.Wait(testContext(t)); err != nil {
		t.Fatalf("delayed entity task failed after move: %v", err)
	}
}

func TestDoRecordsPanic(t *testing.T) {
	w := New()
	defer w.Close()

	task := w.Do(func(tx *Tx) { panic("boom") })
	err := task.Wait(testContext(t))
	if !errors.Is(err, ErrTaskPanicked) {
		t.Fatalf("expected ErrTaskPanicked, got %v", err)
	}
	panicErr, ok := errors.AsType[*PanicError](err)
	if !ok {
		t.Fatalf("expected *PanicError, got %T", err)
	}
	if len(panicErr.Stack) == 0 {
		t.Fatal("expected PanicError to capture the panicking goroutine's stack")
	}
}

// TestDoLogsRecoveredPanic verifies that a recovered fire-and-forget panic is
// reported through the world's logger.
func TestDoLogsRecoveredPanic(t *testing.T) {
	var logs bytes.Buffer
	w := Config{Log: slog.New(slog.NewTextHandler(&logs, nil))}.New()
	defer w.Close()

	task := w.Do(func(tx *Tx) { panic("boom") })
	if err := task.Wait(testContext(t)); !errors.Is(err, ErrTaskPanicked) {
		t.Fatalf("expected ErrTaskPanicked, got %v", err)
	}
	if got := strings.Count(logs.String(), "level=ERROR"); got != 1 {
		t.Fatalf("expected one Error-level panic log, got %v: %q", got, logs.String())
	}
	if !strings.Contains(logs.String(), "boom") {
		t.Fatalf("expected panic log to mention panic value: %q", logs.String())
	}
}

func TestRethrowPanicPanicsWithOriginalValue(t *testing.T) {
	const value = "boom"
	defer func() {
		if v := recover(); v != value {
			t.Fatalf("expected panic value %q, got %v", value, v)
		}
	}()

	RethrowPanic(fmt.Errorf("packet: %w", &PanicError{Value: value}))
}

func TestRethrowPanicIgnoresOrdinaryError(t *testing.T) {
	RethrowPanic(fmt.Errorf("ordinary error"))
	RethrowPanic(nil)
}

func TestTaskZeroValue(t *testing.T) {
	tasks := map[string]*Task{
		"value": new(Task),
		"nil":   nil,
	}
	for name, task := range tasks {
		t.Run(name, func(t *testing.T) {
			select {
			case <-task.Done():
			default:
				t.Fatal("Done channel was not closed")
			}
			if err := task.Err(); !errors.Is(err, ErrTaskCancelled) {
				t.Fatalf("expected ErrTaskCancelled from Err, got %v", err)
			}
			if err := task.Wait(testContext(t)); !errors.Is(err, ErrTaskCancelled) {
				t.Fatalf("expected ErrTaskCancelled from Wait, got %v", err)
			}
			if task.Cancel() {
				t.Fatal("Cancel reported that a zero-value task was cancelled")
			}
		})
	}
}

func TestTaskOnDoneAlwaysRunsAsynchronously(t *testing.T) {
	tasks := map[string]*Task{
		"zero": new(Task),
		"nil":  nil,
	}
	for name, task := range tasks {
		t.Run(name, func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			returned := make(chan struct{})
			go func() {
				task.OnDone(func(err error) {
					if !errors.Is(err, ErrTaskCancelled) {
						t.Errorf("expected ErrTaskCancelled, got %v", err)
					}
					close(started)
					<-release
				})
				close(returned)
			}()
			<-started
			select {
			case <-returned:
			case <-time.After(time.Second):
				close(release)
				t.Fatal("OnDone did not return before its callback completed")
			}
			close(release)
		})
	}
}

func TestDeferAfterTransactionFinishesPanics(t *testing.T) {
	w := Config{Synchronous: true}.New()
	defer w.Close()

	var tx *Tx
	w.Do(func(current *Tx) { tx = current })

	defer func() {
		const expected = "world.Tx: use of transaction after transaction finishes is not permitted"
		if got := recover(); got != expected {
			t.Fatalf("expected panic %q, got %v", expected, got)
		}
	}()
	tx.Defer(func(*Tx) {})
}

func TestEntityHandleClosed(t *testing.T) {
	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	if h.Closed() {
		t.Fatal("new handle reported closed")
	}
	if err := h.Close(); err != nil {
		t.Fatalf("close entity: %v", err)
	}
	if !h.Closed() {
		t.Fatal("closed handle did not report closed")
	}
}

func TestDoRunsDeferredWork(t *testing.T) {
	w := New()
	defer w.Close()

	ran := make(chan struct{})
	var nested *Task
	task := w.Do(func(tx *Tx) {
		tx.Defer(func(tx *Tx) {
			nested = tx.Defer(func(*Tx) { close(ran) })
		})
	})
	if err := task.Wait(testContext(t)); err != nil {
		t.Fatalf("scheduled task failed: %v", err)
	}
	if nested == nil {
		t.Fatal("nested deferred task was not created")
	}
	if err := nested.Wait(testContext(t)); err != nil {
		t.Fatalf("nested deferred task failed: %v", err)
	}
	select {
	case <-ran:
	default:
		t.Fatal("deferred work did not run")
	}
}

func TestDeferErrRecordsCallbackError(t *testing.T) {
	w := New()
	defer w.Close()

	errDeferred := errors.New("deferred error")
	var deferred *Task
	task := w.Do(func(tx *Tx) {
		deferred = tx.DeferErr(func(*Tx) error { return errDeferred })
	})
	if err := task.Wait(testContext(t)); err != nil {
		t.Fatalf("scheduled task failed: %v", err)
	}
	if deferred == nil {
		t.Fatal("deferred task was not created")
	}
	if err := deferred.Wait(testContext(t)); !errors.Is(err, errDeferred) {
		t.Fatalf("expected deferred error, got %v", err)
	}
}

func TestEntityDoWaitsForDeferredWork(t *testing.T) {
	w := New()
	defer w.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	release := make(chan struct{})
	var deferredRan atomic.Bool
	task := h.Do(func(tx *Tx, _ Entity) {
		tx.Defer(func(*Tx) {
			<-release
			deferredRan.Store(true)
		})
	})

	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	if err := task.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		cancel()
		t.Fatalf("expected task to wait for deferred work, got %v", err)
	}
	cancel()

	close(release)
	if err := task.Wait(testContext(t)); err != nil {
		t.Fatalf("entity scheduled task failed: %v", err)
	}
	if !deferredRan.Load() {
		t.Fatal("deferred entity work did not run")
	}
}

func TestEntityDoCancelAfterInvalidatedWeakTransactionDoesNotPoisonHandle(t *testing.T) {
	w := New()
	defer w.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	started := make(chan struct{})
	release := make(chan struct{})
	w.exec(func(*Tx) {
		close(started)
		<-release
	})
	<-started

	removeDone := w.exec(func(tx *Tx) {
		e, ok := h.Entity(tx)
		if !ok {
			t.Error("entity missing before remove")
			return
		}
		tx.RemoveEntity(e)
	})
	task := h.Do(func(*Tx, Entity) {
		t.Error("cancelled task ran")
	})
	// The fast path in schedule may queue directly without a weak
	// transaction. Either way, the task must still be cancellable while
	// pending.
	if !task.Cancel() {
		t.Fatal("expected pending task to cancel")
	}
	close(release)
	<-removeDone
	if err := task.Wait(testContext(t)); !errors.Is(err, ErrTaskCancelled) {
		t.Fatalf("expected ErrTaskCancelled, got %v", err)
	}

	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })
	task = h.Do(func(*Tx, Entity) {})
	if err := task.Wait(testContext(t)); err != nil {
		t.Fatalf("handle poisoned after cancelling invalidated weak transaction: %v", err)
	}
}

func TestDoDoesNotBlockOwnerWhenQueueFull(t *testing.T) {
	w := New()
	defer w.Close()

	done := make(chan struct{})
	go func() {
		<-w.exec(func(tx *Tx) {
			for i := 0; i < cap(w.queue)+32; i++ {
				w.Do(func(*Tx) {})
				tx.Defer(func(*Tx) {})
			}
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Do or Context.Defer blocked the world owner")
	}
}

// TestWeakExecDoesNotBlockOwnerWhenQueueFull ensures an off-owner weak
// transaction blocked on a full queue does not hold scheduleMu, which would
// deadlock an owner callback calling World.Do.
func TestWeakExecDoesNotBlockOwnerWhenQueueFull(t *testing.T) {
	w := New()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	entered := make(chan struct{})
	proceed := make(chan struct{})
	release := make(chan struct{})

	// Occupy the world owner goroutine.
	w.exec(func(tx *Tx) {
		close(entered)
		<-proceed
		// Owner-side fire-and-forget must not block on scheduleMu.
		w.Do(func(tx *Tx) {})
		close(release)
	})
	<-entered

	// Fill the queue to capacity while the owner is busy.
	for i := 0; i < cap(w.queue); i++ {
		w.queue <- normalTransaction{c: make(chan struct{}), f: func(tx *Tx) {}}
	}

	// Entity task whose weak transaction ends up in World.weakExec with the
	// queue full.
	task := h.Do(func(tx *Tx, e Entity) {})
	time.Sleep(200 * time.Millisecond)

	close(proceed)
	select {
	case <-release:
	case <-time.After(3 * time.Second):
		t.Fatal("owner-side World.Do deadlocked on scheduleMu held by weakExec blocked on full queue")
	}
	if err := task.Wait(testContext(t)); err != nil {
		t.Fatalf("entity task failed after queue drained: %v", err)
	}
	_ = w.Close()
}

func TestDoQueuedBeforeCloseDoesNotRunAfterHandleClose(t *testing.T) {
	w := New()

	var closeHandled atomic.Bool
	var ranAfterClose atomic.Bool
	w.Handle(closeOrderHandler{closed: &closeHandled})

	started := make(chan struct{})
	release := make(chan struct{})
	w.exec(func(*Tx) {
		close(started)
		<-release
	})
	<-started

	for i := 0; i < cap(w.queue); i++ {
		w.exec(func(*Tx) {})
	}
	task := w.Do(func(*Tx) {
		if closeHandled.Load() {
			ranAfterClose.Store(true)
		}
	})

	closed := make(chan struct{})
	go func() {
		_ = w.Close()
		close(closed)
	}()
	close(release)

	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("world close did not complete")
	}
	if err := task.Wait(testContext(t)); err != nil && !errors.Is(err, ErrWorldClosed) {
		t.Fatalf("scheduled task failed with unexpected error: %v", err)
	}
	if ranAfterClose.Load() {
		t.Fatal("scheduled task ran after HandleClose")
	}
}

func TestHandleCloseDrainsDeferredWorkBeforeSave(t *testing.T) {
	var deferredRan atomic.Bool
	provider := &closeDeferredProvider{deferredRan: &deferredRan}
	w := Config{Provider: provider}.New()
	w.Handle(closeDeferredHandler{deferredRan: &deferredRan})

	if err := w.Close(); err != nil {
		t.Fatalf("close world: %v", err)
	}
	if !deferredRan.Load() {
		t.Fatal("HandleClose deferred work did not run")
	}
	if provider.savedBeforeDeferred.Load() {
		t.Fatal("world saved before HandleClose deferred work ran")
	}
}

func TestDoAfterEntityCloseFailsPromptly(t *testing.T) {
	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	task := h.DoAfter(time.Hour, func(*Tx, Entity) {
		t.Error("delayed task ran after entity closed")
	})
	if err := h.Close(); err != nil {
		t.Fatalf("close entity: %v", err)
	}
	if err := task.Wait(testContext(t)); !errors.Is(err, ErrEntityClosed) {
		t.Fatalf("expected ErrEntityClosed, got %v", err)
	}
}

func TestEntityDoCompletesWhenEntityClosesBeforeQueuedTask(t *testing.T) {
	w := New()
	defer w.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	started := make(chan struct{})
	release := make(chan struct{})
	done := w.exec(func(tx *Tx) {
		close(started)
		<-release
		e, ok := h.Entity(tx)
		if !ok {
			t.Error("entity missing")
			return
		}
		tx.RemoveEntity(e)
		_ = h.Close()
	})
	<-started

	task := h.Do(func(*Tx, Entity) { t.Error("closed entity task ran") })
	time.Sleep(50 * time.Millisecond)
	close(release)
	<-done

	if err := task.Wait(testContext(t)); !errors.Is(err, ErrEntityClosed) {
		t.Fatalf("expected ErrEntityClosed, got %v", err)
	}
}

func TestExecWorldReturnsTrueWhenCallbackClosesEntity(t *testing.T) {
	w := New()
	defer w.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	ran := false
	ok := h.execWorld(func(tx *Tx, e Entity) {
		ran = true
		tx.RemoveEntity(e)
		_ = h.Close()
	}, false, nil, nil)
	if !ran {
		t.Fatal("execWorld callback did not run")
	}
	if !ok {
		t.Fatal("execWorld returned false after callback ran")
	}
}

func TestDoAfterWorldCloseFails(t *testing.T) {
	w := New()
	if err := w.Close(); err != nil {
		t.Fatalf("close world: %v", err)
	}

	task := w.DoAfter(time.Hour, func(tx *Tx) { t.Error("task ran on closed world") })
	if err := task.Wait(testContext(t)); !errors.Is(err, ErrWorldClosed) {
		t.Fatalf("expected ErrWorldClosed, got %v", err)
	}
}

func TestEntityDoScheduledDuringWorldCloseRunsBeforeQueueShutdown(t *testing.T) {
	var task *Task
	w := New()
	h := NewEntity(closeSchedulingEntityType{}, closeSchedulingEntityConfig{onClose: func(h *EntityHandle) {
		task = h.Do(func(*Tx, Entity) {})
	}})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	if err := w.Close(); err != nil {
		t.Fatalf("close world: %v", err)
	}
	if task == nil {
		t.Fatal("entity close did not schedule cleanup task")
	}
	if err := task.Wait(testContext(t)); err != nil {
		t.Fatalf("close-time entity task failed: %v", err)
	}
}

func TestEntityDoBlockedBeforeWorldCloseFailsPromptly(t *testing.T) {
	w := New()
	h := NewEntity(closeSchedulingEntityType{}, closeSchedulingEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	h.cond.L.Lock()
	h.weakTxActive = true
	h.cond.L.Unlock()
	task := h.Do(func(*Tx, Entity) {
		t.Error("entity task ran after world close")
	})

	if err := w.Close(); err != nil {
		t.Fatalf("close world: %v", err)
	}
	h.cond.L.Lock()
	h.weakTxActive = false
	h.cond.Broadcast()
	h.cond.L.Unlock()

	if err := task.Wait(testContext(t)); !errors.Is(err, ErrWorldClosed) {
		t.Fatalf("expected ErrWorldClosed, got %v", err)
	}
}

func TestEntityDoAfterWorldCloseFails(t *testing.T) {
	w := New()
	h := NewEntity(closeSchedulingEntityType{}, closeSchedulingEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })
	if err := w.Close(); err != nil {
		t.Fatalf("close world: %v", err)
	}

	task := h.Do(func(*Tx, Entity) {
		t.Error("entity task ran after world close")
	})
	if err := task.Wait(testContext(t)); !errors.Is(err, ErrWorldClosed) {
		t.Fatalf("expected ErrWorldClosed, got %v", err)
	}
}

// TestEventCancellationIsolated verifies that Context.Event produces
// independent cancellation scopes: cancelling one dispatched event must not
// leak to another event or to the owning transaction, even though they all
// share the same transaction. This is what lets many block handlers fire and
// cancel their own world events within a single tick transaction without
// interfering with one another.
func TestEventCancellationIsolated(t *testing.T) {
	w := New()
	defer w.Close()

	<-w.exec(func(tx *Tx) {
		first, second := tx.Event(), tx.Event()
		first.Cancel()

		if !first.Cancelled() {
			t.Error("cancelled event does not report Cancelled")
		}
		if second.Cancelled() {
			t.Error("cancelling one event leaked into another event of the same transaction")
		}
		// The events must still share the underlying transaction so world
		// operations from a handler reach the same world.
		if first.Tx != tx || second.Tx != tx || first.World() != tx.World() || second.World() != tx.World() {
			t.Error("event contexts do not share the transaction's world")
		}
	})
}

type taskTestEntityConfig struct{}

type closeOrderHandler struct {
	NopHandler
	closed *atomic.Bool
}

func (h closeOrderHandler) HandleClose(*Tx) { h.closed.Store(true) }

type closeDeferredHandler struct {
	NopHandler
	deferredRan *atomic.Bool
}

func (h closeDeferredHandler) HandleClose(tx *Tx) {
	tx.Defer(func(*Tx) { h.deferredRan.Store(true) })
}

type closeDeferredProvider struct {
	NopProvider
	deferredRan         *atomic.Bool
	savedBeforeDeferred atomic.Bool
}

func (p *closeDeferredProvider) SaveSettings(*Settings) {
	if !p.deferredRan.Load() {
		p.savedBeforeDeferred.Store(true)
	}
}

type closeWaitTaskHandler struct {
	NopHandler
	task *Task
	errc chan error
}

func (h closeWaitTaskHandler) HandleClose(*Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	h.errc <- h.task.Wait(ctx)
}

type closeSchedulingEntityConfig struct {
	onClose func(*EntityHandle)
}

func (c closeSchedulingEntityConfig) Apply(data *EntityData) { data.Data = c.onClose }

type closeSchedulingEntityType struct{}

func (closeSchedulingEntityType) Open(_ *Tx, handle *EntityHandle, data *EntityData) Entity {
	onClose, _ := data.Data.(func(*EntityHandle))
	return closeSchedulingEntity{h: handle, onClose: onClose}
}

func (closeSchedulingEntityType) EncodeEntity() string { return "dragonfly:close_scheduling_entity" }

func (closeSchedulingEntityType) BBox(Entity) cube.BBox { return cube.BBox{} }

func (closeSchedulingEntityType) DecodeNBT(map[string]any, *EntityData) {}

func (closeSchedulingEntityType) EncodeNBT(*EntityData) map[string]any { return nil }

type closeSchedulingEntity struct {
	h       *EntityHandle
	onClose func(*EntityHandle)
}

func (e closeSchedulingEntity) Close() error {
	if e.onClose != nil {
		e.onClose(e.h)
	}
	return nil
}

func (e closeSchedulingEntity) H() *EntityHandle { return e.h }

func (closeSchedulingEntity) Position() mgl64.Vec3 { return mgl64.Vec3{} }

func (closeSchedulingEntity) Rotation() cube.Rotation { return cube.Rotation{} }

func (taskTestEntityConfig) Apply(*EntityData) {}

type taskTestEntityType struct{}

func (taskTestEntityType) Open(tx *Tx, handle *EntityHandle, _ *EntityData) Entity {
	return taskTestEntity{h: handle, tx: tx}
}

func (taskTestEntityType) EncodeEntity() string { return "dragonfly:test_entity" }

func (taskTestEntityType) BBox(Entity) cube.BBox { return cube.BBox{} }

func (taskTestEntityType) DecodeNBT(map[string]any, *EntityData) {}

func (taskTestEntityType) EncodeNBT(*EntityData) map[string]any { return nil }

type taskTestEntity struct {
	h  *EntityHandle
	tx *Tx
}

type markedTaskEntity interface {
	Entity
	markedTaskEntity()
}

func (e taskTestEntity) Close() error {
	if e.tx != nil {
		if ent, ok := e.h.Entity(e.tx); ok {
			e.tx.RemoveEntity(ent)
		}
	}
	return e.h.Close()
}

func (e taskTestEntity) H() *EntityHandle { return e.h }

func (taskTestEntity) Position() mgl64.Vec3 { return mgl64.Vec3{} }

func (taskTestEntity) Rotation() cube.Rotation { return cube.Rotation{} }

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}
