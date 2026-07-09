package player

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/player/skin"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestRespawnCompletesInSynchronousWorld(t *testing.T) {
	conn := &testConn{}
	sess := session.Config{HandleStop: func(*world.Tx, session.Controllable) {}}.New(conn)
	w := world.Config{
		Synchronous: true,
		Entities:    world.EntityRegistryConfig{}.New([]world.EntityType{Type}),
	}.New()
	h := world.EntitySpawnOpts{}.New(Type, Config{Session: sess})

	done := make(chan struct{})
	go func() {
		w.Do(func(tx *world.Tx) {
			p := tx.AddEntity(h).(*Player)
			p.addHealth(-p.MaxHealth())
			p.respawn(nil)
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("respawn deadlocked in synchronous world")
	}

	w.Do(func(tx *world.Tx) {
		if e, ok := h.Entity(tx); ok {
			e.(*Player).s = nil
		}
	})
	sess.CloseConnection()
	if err := w.Close(); err != nil {
		t.Fatalf("close world: %v", err)
	}
}

func TestRespawnClosedDestinationFallsBackBeforeReturning(t *testing.T) {
	conn := &testConn{}
	sess := session.Config{HandleStop: func(*world.Tx, session.Controllable) {}}.New(conn)
	registry := world.EntityRegistryConfig{}.New([]world.EntityType{Type})
	src := world.Config{Synchronous: true, Entities: registry}.New()
	dst := world.Config{Synchronous: true, Entities: registry}.New()
	if err := dst.Close(); err != nil {
		t.Fatalf("close destination world: %v", err)
	}
	h := world.EntitySpawnOpts{}.New(Type, Config{Session: sess})

	started := make(chan struct{})
	release := make(chan struct{})
	returned := make(chan struct{})
	go func() {
		src.Do(func(tx *world.Tx) {
			p := tx.AddEntity(h).(*Player)
			p.Handle(redirectRespawnHandler{w: dst})
			p.addHealth(-p.MaxHealth())
			p.respawn(func(np *Player) {
				close(started)
				<-release
				np.s = nil
			})
		})
		close(returned)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("closed-destination fallback did not start")
	}
	select {
	case <-returned:
		t.Fatal("respawn returned before closed-destination fallback completed")
	default:
	}
	close(release)
	<-returned

	sess.CloseConnection()
	if err := src.Close(); err != nil {
		t.Fatalf("close source world: %v", err)
	}
}

// TestOrphanedSessionCloseRunsStopHandler covers the respawn fallback for a
// player whose destination and source worlds have both closed: the session is
// closed with a nil transaction, which must still run the stop handler so the
// server can balance its player accounting.
func TestOrphanedSessionCloseRunsStopHandler(t *testing.T) {
	conn := &testConn{}
	stopped := make(chan *world.Tx, 1)
	sess := session.Config{HandleStop: func(tx *world.Tx, _ session.Controllable) {
		stopped <- tx
	}}.New(conn)
	w := world.Config{
		Synchronous: true,
		Entities:    world.EntityRegistryConfig{}.New([]world.EntityType{Type}),
	}.New()
	h := world.EntitySpawnOpts{}.New(Type, Config{Session: sess})
	sess.SetHandle(h, skin.Skin{})

	var p *Player
	w.Do(func(tx *world.Tx) {
		p = tx.AddEntity(h).(*Player)
		tx.RemoveEntity(p)
	})
	if err := w.Close(); err != nil {
		t.Fatalf("close world: %v", err)
	}

	_ = h.Close()
	sess.Disconnect("respawn failed")
	sess.Close(nil, p)
	sess.CloseConnection()

	select {
	case tx := <-stopped:
		if tx != nil {
			t.Fatal("expected nil transaction in stop handler for orphaned session close")
		}
	default:
		t.Fatal("stop handler did not run for orphaned session close")
	}
}

func TestContextDeferErrReturnsCallbackError(t *testing.T) {
	w := newSynchronousTestWorld()
	defer w.Close()
	h := world.EntitySpawnOpts{}.New(Type, Config{})
	want := errors.New("deferred player error")

	var deferred *world.Task
	w.Do(func(tx *world.Tx) {
		p := tx.AddEntity(h).(*Player)
		deferred = newContext(p).DeferErr(func(ctx *Context) error {
			if ctx.Player().H() != h {
				t.Error("deferred context resolved the wrong player")
			}
			return want
		})
	})
	if err := deferred.Wait(context.Background()); !errors.Is(err, want) {
		t.Fatalf("expected deferred callback error, got %v", err)
	}
}

func TestContextDeferErrReportsEntityNotInWorld(t *testing.T) {
	src := newSynchronousTestWorld()
	defer src.Close()
	dst := newSynchronousTestWorld()
	defer dst.Close()
	h := world.EntitySpawnOpts{}.New(Type, Config{})

	var deferred *world.Task
	src.Do(func(tx *world.Tx) {
		p := tx.AddEntity(h).(*Player)
		deferred = newContext(p).DeferErr(func(*Context) error {
			t.Error("deferred callback ran after player changed worlds")
			return nil
		})
		tx.RemoveEntity(p)
		dst.Do(func(tx *world.Tx) { tx.AddEntity(h) })
	})
	if err := deferred.Wait(context.Background()); !errors.Is(err, world.ErrEntityNotInWorld) {
		t.Fatalf("expected ErrEntityNotInWorld, got %v", err)
	}
}

func TestContextDeferErrReportsClosedEntity(t *testing.T) {
	w := newSynchronousTestWorld()
	defer w.Close()
	h := world.EntitySpawnOpts{}.New(Type, Config{})

	var deferred *world.Task
	w.Do(func(tx *world.Tx) {
		p := tx.AddEntity(h).(*Player)
		deferred = newContext(p).DeferErr(func(*Context) error {
			t.Error("deferred callback ran after player handle closed")
			return nil
		})
		tx.RemoveEntity(p)
		if err := h.Close(); err != nil {
			t.Fatalf("close handle: %v", err)
		}
	})
	if err := deferred.Wait(context.Background()); !errors.Is(err, world.ErrEntityClosed) {
		t.Fatalf("expected ErrEntityClosed, got %v", err)
	}
}

func TestOneShotPlayerHelpers(t *testing.T) {
	w := newSynchronousTestWorld()
	defer w.Close()
	h := world.EntitySpawnOpts{}.New(Type, Config{})
	w.Do(func(tx *world.Tx) { tx.AddEntity(h) })

	var didRun bool
	if err := Do(h, func(_ *world.Tx, p *Player) { didRun = p.H() == h }).Wait(context.Background()); err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	if !didRun {
		t.Fatal("Do did not resolve the player")
	}

	didRun = false
	if err := DoAfter(h, time.Millisecond, func(_ *world.Tx, p *Player) { didRun = p.H() == h }).Wait(context.Background()); err != nil {
		t.Fatalf("DoAfter failed: %v", err)
	}
	if !didRun {
		t.Fatal("DoAfter did not resolve the player")
	}

	got, err := Call(context.Background(), h, func(_ *world.Tx, p *Player) (bool, error) {
		return p.H() == h, nil
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if !got {
		t.Fatal("Call did not resolve the player")
	}
}

func newSynchronousTestWorld() *world.World {
	return world.Config{
		Synchronous: true,
		Entities:    world.EntityRegistryConfig{}.New([]world.EntityType{Type}),
	}.New()
}

type redirectRespawnHandler struct {
	NopHandler
	w *world.World
}

func (h redirectRespawnHandler) HandleRespawn(_ *Player, _ *mgl64.Vec3, w **world.World) {
	*w = h.w
}

type testConn struct{}

func (*testConn) Close() error                                               { return nil }
func (*testConn) IdentityData() login.IdentityData                           { return login.IdentityData{} }
func (*testConn) ClientData() login.ClientData                               { return login.ClientData{} }
func (*testConn) ClientCacheEnabled() bool                                   { return false }
func (*testConn) ChunkRadius() int                                           { return 0 }
func (*testConn) Latency() time.Duration                                     { return 0 }
func (*testConn) Flush() error                                               { return nil }
func (*testConn) RemoteAddr() net.Addr                                       { return &net.IPAddr{} }
func (*testConn) ReadPacket() (packet.Packet, error)                         { return nil, io.EOF }
func (*testConn) WritePacket(packet.Packet) error                            { return nil }
func (*testConn) StartGameContext(context.Context, minecraft.GameData) error { return nil }
