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

func TestDoRunsOnWorldContext(t *testing.T) {
	w := New()
	defer w.Close()

	task := w.Do(func(ctx *Context) {
		if ctx.Tx() == nil {
			t.Fatal("scheduled context has nil transaction")
		}
		if ctx.Tx() == nil {
			t.Fatal("Context.Tx returned nil")
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
		return ctx.Tx().CurrentTick(), nil
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

func TestCallEntityReturnsTypedResult(t *testing.T) {
	w := New()
	defer w.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	got, err := CallEntity(testContext(t), h, func(ctx *Context, e Entity) (mgl64.Vec3, error) {
		if ctx.Tx() == nil {
			t.Fatal("entity call context has nil transaction")
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
	_, err := CallEntity(ctx, h, func(ctx *Context, e Entity) (int, error) {
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

	got, err := CallRef(testContext(t), NewEntityRef[taskTestEntity](h), func(ctx *Context, e taskTestEntity) (bool, error) {
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

	_, err := CallRef(testContext(t), NewEntityRef[markedTaskEntity](h), func(ctx *Context, e markedTaskEntity) (int, error) {
		t.Fatal("CallRef ran typed callback for mismatched entity")
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
	task := w.DoAfter(time.Hour, func(ctx *Context) { close(ran) })
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

	task := w.DoAfter(time.Hour, func(*Context) {
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

func TestEntityDoAfterFailsWhenWorldCloseStarts(t *testing.T) {
	w := New()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	task := h.DoAfter(time.Hour, func(*Context, Entity) {
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

func TestEntityDoAfterFollowsMovedEntity(t *testing.T) {
	w1 := New()
	w2 := New()
	defer w2.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w1.exec(func(tx *Tx) { tx.AddEntity(h) })

	task := h.DoAfter(50*time.Millisecond, func(ctx *Context, _ Entity) {
		if ctx.Tx().World() != w2 {
			t.Fatal("delayed entity task did not follow moved entity")
		}
	})
	<-w1.exec(func(tx *Tx) {
		e, ok := h.Entity(tx)
		if !ok {
			t.Fatal("entity missing before move")
		}
		tx.RemoveEntity(e)
	})
	<-w2.exec(func(tx *Tx) { tx.AddEntity(h) })
	if err := w1.Close(); err != nil {
		t.Fatalf("close old world: %v", err)
	}
	if err := task.Wait(testContext(t)); err != nil {
		t.Fatalf("delayed entity task failed after move: %v", err)
	}
}

func TestDoRecordsPanic(t *testing.T) {
	w := New()
	defer w.Close()

	task := w.Do(func(ctx *Context) { panic("boom") })
	if err := task.Wait(testContext(t)); !errors.Is(err, ErrTaskPanicked) {
		t.Fatalf("expected ErrTaskPanicked, got %v", err)
	}
}

func TestDoRunsDeferredWork(t *testing.T) {
	w := New()
	defer w.Close()

	ran := make(chan struct{})
	var nested *Task
	task := w.Do(func(ctx *Context) {
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

func TestEntityDoWaitsForDeferredWork(t *testing.T) {
	w := New()
	defer w.Close()

	h := NewEntity(taskTestEntityType{}, taskTestEntityConfig{})
	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })

	release := make(chan struct{})
	var deferredRan atomic.Bool
	task := h.Do(func(ctx *Context, _ Entity) {
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
			t.Fatal("entity missing before remove")
		}
		tx.RemoveEntity(e)
	})
	task := h.Do(func(*Context, Entity) {
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

	<-w.exec(func(tx *Tx) { tx.AddEntity(h) })
	task = h.Do(func(*Context, Entity) {})
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
			ctx := tx.Context()
			for i := 0; i < cap(w.queue)+32; i++ {
				w.Do(func(*Context) {})
				ctx.Defer(func(*Context) {})
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
	task := w.Do(func(*Context) {
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
	task := h.DoAfter(time.Hour, func(*Context, Entity) {
		t.Fatal("delayed task ran after entity closed")
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
			t.Fatal("entity missing")
		}
		tx.RemoveEntity(e)
		_ = h.Close()
	})
	<-started

	task := h.Do(func(*Context, Entity) { t.Fatal("closed entity task ran") })
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
	}, false, nil)
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

	task := w.DoAfter(time.Hour, func(ctx *Context) { t.Fatal("task ran on closed world") })
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

type closeDeferredHandler struct {
	NopHandler
	deferredRan *atomic.Bool
}

func (h closeDeferredHandler) HandleClose(ctx *Context) {
	ctx.Defer(func(*Context) { h.deferredRan.Store(true) })
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
