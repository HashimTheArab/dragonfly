package world

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

func TestScheduleRunsOnWorldContext(t *testing.T) {
	w := New()
	defer w.Close()

	task := w.Schedule(func(ctx *Context) {
		if ctx.Tx() == nil {
			t.Fatal("scheduled context has nil transaction")
		}
		if ctx.World() != ctx.Tx() {
			t.Fatal("Context.World does not return owner-scoped transaction")
		}
	})
	if err := task.Wait(testContext(t)); err != nil {
		t.Fatalf("scheduled task failed: %v", err)
	}
}

func TestCallReturnsTypedResult(t *testing.T) {
	w := New()
	defer w.Close()

	got, err := Call(testContext(t), w, func(ctx *Context) (int64, error) {
		return ctx.CurrentTick(), nil
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if got < 0 {
		t.Fatalf("Call returned negative tick: %v", got)
	}
}

func TestCallDoesNotScheduleCancelledContext(t *testing.T) {
	w := New()
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var ran atomic.Bool
	_, err := Call(ctx, w, func(ctx *Context) (int, error) {
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

func TestScheduleAfterCancel(t *testing.T) {
	w := New()
	defer w.Close()

	ran := make(chan struct{})
	task := w.ScheduleAfter(time.Hour, func(ctx *Context) { close(ran) })
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

func TestScheduleAfterFailsWhenWorldCloseStarts(t *testing.T) {
	w := New()

	task := w.ScheduleAfter(time.Hour, func(*Context) {
		t.Fatal("delayed task ran after world close started")
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

func TestEntityScheduleAfterFailsWhenWorldCloseStarts(t *testing.T) {
	w := New()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.Exec(func(tx *Tx) { tx.AddEntity(h) })

	task := h.ScheduleAfter(time.Hour, func(*Context, Entity) {
		t.Fatal("delayed entity task ran after world close started")
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

func TestEntityScheduleAfterFollowsMovedEntity(t *testing.T) {
	w1 := New()
	w2 := New()
	defer w2.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w1.Exec(func(tx *Tx) { tx.AddEntity(h) })

	task := h.ScheduleAfter(50*time.Millisecond, func(ctx *Context, _ Entity) {
		if ctx.Tx().World() != w2 {
			t.Fatal("delayed entity task did not follow moved entity")
		}
	})
	<-w1.Exec(func(tx *Tx) {
		e, ok := h.Entity(tx)
		if !ok {
			t.Fatal("entity missing before move")
		}
		tx.RemoveEntity(e)
	})
	<-w2.Exec(func(tx *Tx) { tx.AddEntity(h) })
	if err := w1.Close(); err != nil {
		t.Fatalf("close old world: %v", err)
	}
	if err := task.Wait(testContext(t)); err != nil {
		t.Fatalf("delayed entity task failed after move: %v", err)
	}
}

func TestScheduleRecordsPanic(t *testing.T) {
	w := New()
	defer w.Close()

	task := w.Schedule(func(ctx *Context) { panic("boom") })
	if err := task.Wait(testContext(t)); !errors.Is(err, ErrTaskPanicked) {
		t.Fatalf("expected ErrTaskPanicked, got %v", err)
	}
}

func TestScheduleRunsDeferredWork(t *testing.T) {
	w := New()
	defer w.Close()

	ran := make(chan struct{})
	var nested *Task
	task := w.Schedule(func(ctx *Context) {
		ctx.Defer(func(ctx *Context) {
			nested = ctx.Defer(func(*Context) { close(ran) })
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

func TestEntityScheduleWaitsForDeferredWork(t *testing.T) {
	w := New()
	defer w.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.Exec(func(tx *Tx) { tx.AddEntity(h) })

	release := make(chan struct{})
	var deferredRan atomic.Bool
	task := h.Schedule(func(ctx *Context, _ Entity) {
		ctx.Defer(func(*Context) {
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

func TestEntityScheduleCancelAfterInvalidatedWeakTransactionDoesNotPoisonHandle(t *testing.T) {
	w := New()
	defer w.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.Exec(func(tx *Tx) { tx.AddEntity(h) })

	started := make(chan struct{})
	release := make(chan struct{})
	w.Exec(func(*Tx) {
		close(started)
		<-release
	})
	<-started

	removeDone := w.Exec(func(tx *Tx) {
		e, ok := h.Entity(tx)
		if !ok {
			t.Fatal("entity missing before remove")
		}
		tx.RemoveEntity(e)
	})
	task := h.Schedule(func(*Context, Entity) {
		t.Fatal("cancelled task ran")
	})
	waitEntityWeakTxActive(t, h)
	if !task.Cancel() {
		t.Fatal("expected pending task to cancel")
	}
	close(release)
	<-removeDone
	if err := task.Wait(testContext(t)); !errors.Is(err, ErrTaskCancelled) {
		t.Fatalf("expected ErrTaskCancelled, got %v", err)
	}

	<-w.Exec(func(tx *Tx) { tx.AddEntity(h) })
	task = h.Schedule(func(*Context, Entity) {})
	if err := task.Wait(testContext(t)); err != nil {
		t.Fatalf("handle poisoned after cancelling invalidated weak transaction: %v", err)
	}
}

func TestScheduleDoesNotBlockOwnerWhenQueueFull(t *testing.T) {
	w := New()
	defer w.Close()

	done := make(chan struct{})
	go func() {
		<-w.Exec(func(tx *Tx) {
			ctx := tx.Context()
			for i := 0; i < cap(w.queue)+32; i++ {
				w.Schedule(func(*Context) {})
				ctx.Defer(func(*Context) {})
			}
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Schedule or Context.Defer blocked the world owner")
	}
}

func TestScheduleQueuedBeforeCloseDoesNotRunAfterHandleClose(t *testing.T) {
	w := New()

	var closeHandled atomic.Bool
	var ranAfterClose atomic.Bool
	w.Handle(closeOrderHandler{closed: &closeHandled})

	started := make(chan struct{})
	release := make(chan struct{})
	w.Exec(func(*Tx) {
		close(started)
		<-release
	})
	<-started

	for i := 0; i < cap(w.queue); i++ {
		w.Exec(func(*Tx) {})
	}
	task := w.Schedule(func(*Context) {
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

func TestScheduleAfterEntityCloseFailsPromptly(t *testing.T) {
	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	task := h.ScheduleAfter(time.Hour, func(*Context, Entity) {
		t.Fatal("delayed task ran after entity closed")
	})
	if err := h.Close(); err != nil {
		t.Fatalf("close entity: %v", err)
	}
	if err := task.Wait(testContext(t)); !errors.Is(err, ErrEntityClosed) {
		t.Fatalf("expected ErrEntityClosed, got %v", err)
	}
}

func TestEntityScheduleCompletesWhenEntityClosesBeforeQueuedTask(t *testing.T) {
	w := New()
	defer w.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.Exec(func(tx *Tx) { tx.AddEntity(h) })

	started := make(chan struct{})
	release := make(chan struct{})
	done := w.Exec(func(tx *Tx) {
		close(started)
		<-release
		e, ok := h.Entity(tx)
		if !ok {
			t.Fatal("entity missing")
		}
		tx.RemoveEntity(e)
		_ = h.Close()
	})
	<-started

	task := h.Schedule(func(*Context, Entity) { t.Fatal("closed entity task ran") })
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
	<-w.Exec(func(tx *Tx) { tx.AddEntity(h) })

	ran := false
	ok := h.ExecWorld(func(tx *Tx, e Entity) {
		ran = true
		tx.RemoveEntity(e)
		_ = h.Close()
	})
	if !ran {
		t.Fatal("ExecWorld callback did not run")
	}
	if !ok {
		t.Fatal("ExecWorld returned false after callback ran")
	}
}

func TestScheduleAfterWorldCloseFails(t *testing.T) {
	w := New()
	if err := w.Close(); err != nil {
		t.Fatalf("close world: %v", err)
	}

	task := w.Schedule(func(ctx *Context) { t.Fatal("task ran on closed world") })
	if err := task.Wait(testContext(t)); !errors.Is(err, ErrWorldClosed) {
		t.Fatalf("expected ErrWorldClosed, got %v", err)
	}
}

type taskTestEntityConfig struct{}

type closeOrderHandler struct {
	NopHandler
	closed *atomic.Bool
}

func (h closeOrderHandler) HandleClose(*Context) { h.closed.Store(true) }

type closeWaitTaskHandler struct {
	NopHandler
	task *Task
	errc chan error
}

func (h closeWaitTaskHandler) HandleClose(*Context) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	h.errc <- h.task.Wait(ctx)
}

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

func waitEntityWeakTxActive(t *testing.T, h *EntityHandle) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		h.cond.L.Lock()
		active := h.weakTxActive
		h.cond.L.Unlock()
		if active {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("entity weak transaction did not become active")
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}
