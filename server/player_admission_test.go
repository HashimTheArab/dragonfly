package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// admissionFixture owns one provider reservation and its latest placement.
type admissionFixture struct {
	player.Provider
	data     player.Config
	w        *world.World
	releases int
	resolves int
	loadErr  error
}

// Save discards the completed fixture player's snapshot.
func (*admissionFixture) Save(uuid.UUID, player.Config, *world.World) error { return nil }

// LoadAdmission returns one login's reservation without using the legacy load path.
func (a *admissionFixture) LoadAdmission(uuid.UUID, func(world.Dimension) *world.World) (player.Admission, error) {
	return a, a.loadErr
}

// Resolve returns the provider's currently valid placement.
func (a *admissionFixture) Resolve() (player.Config, *world.World) { a.resolves++; return a.data, a.w }

// Release records the server's ownership handoff or failure cleanup.
func (a *admissionFixture) Release() { a.releases++ }

// admissionConn provides a synchronous handshake and a closeable packet reader.
type admissionConn struct {
	session.Conn
	id       uuid.UUID
	startErr error
	closed   chan struct{}
	once     sync.Once
}

// IdentityData supplies a valid login identity.
func (c *admissionConn) IdentityData() login.IdentityData {
	return login.IdentityData{Identity: c.id.String(), DisplayName: "Actor"}
}

// ClientData supplies an empty skin and locale.
func (*admissionConn) ClientData() login.ClientData { return login.ClientData{} }

// ChunkRadius keeps the test's loader small.
func (*admissionConn) ChunkRadius() int { return 1 }

// ClientCacheEnabled keeps chunk traffic independent of blob acknowledgments.
func (*admissionConn) ClientCacheEnabled() bool { return false }

// Latency supplies the local fixture's zero latency.
func (*admissionConn) Latency() time.Duration { return 0 }

// RemoteAddr supplies a local address for session logs.
func (*admissionConn) RemoteAddr() net.Addr { return &net.UDPAddr{} }

// StartGameContext completes or fails the handshake as selected by the test.
func (c *admissionConn) StartGameContext(context.Context, minecraft.GameData) error {
	return c.startErr
}

// WritePacket discards packets without scheduling network work.
func (*admissionConn) WritePacket(packet.Packet) error { return nil }

// WritePacketImmediate discards the immediate packet batch.
func (*admissionConn) WritePacketImmediate(...packet.Packet) error { return nil }

// ReadPacket waits for connection teardown without sleeps.
func (c *admissionConn) ReadPacket() (packet.Packet, error) { <-c.closed; return nil, io.EOF }

// Close unblocks all packet readers exactly once.
func (c *admissionConn) Close() error { c.once.Do(func() { close(c.closed) }); return nil }

// admissionListener closes rejected logins without a real socket.
type admissionListener struct{ Listener }

// Disconnect closes the rejected fixture connection.
func (admissionListener) Disconnect(c session.Conn, _ string) error { return c.Close() }

// admissionServer builds only the world and server state needed for login.
func admissionServer(t *testing.T, dimension ...world.Dimension) (*Server, *admissionFixture, *admissionConn) {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	var dim world.Dimension = world.Overworld
	if len(dimension) != 0 {
		dim = dimension[0]
	}
	w := world.Config{Log: log, Dim: dim, Entities: entity.DefaultRegistry}.New()
	t.Cleanup(func() { _ = w.Close() })
	a := &admissionFixture{w: w, data: player.Config{Position: mgl64.Vec3{3, 70, 5}}}
	c := &admissionConn{id: uuid.New(), closed: make(chan struct{})}
	t.Cleanup(func() { _ = c.Close() })
	srv := &Server{conf: Config{Log: log, PlayerProvider: a, MaxChunkRadius: 1, Blocks: world.DefaultBlockRegistry}, world: w, nether: w, end: w, p: make(map[uuid.UUID]*onlinePlayer), incoming: make(chan incoming, 1)}
	return srv, a, c
}

// TestLoginFailureReleasesAdmission covers storage errors, failed handshakes and duplicate logins.
func TestLoginFailureReleasesAdmission(t *testing.T) {
	for _, failure := range []string{"load", "handshake", "duplicate"} {
		t.Run(failure, func(t *testing.T) {
			srv, a, c := admissionServer(t)
			c.startErr = errors.New("handshake failed")
			if failure == "load" {
				a.loadErr = errors.New("load failed")
			}
			if failure == "duplicate" {
				c.startErr = nil
				srv.p[c.id] = &onlinePlayer{}
			}
			srv.finaliseConn(t.Context(), c, admissionListener{})
			if a.releases != 1 || len(srv.incoming) != 0 {
				t.Fatalf("failure cleanup = %d releases, %d pending players", a.releases, len(srv.incoming))
			}
		})
	}
}

// TestLoginRevalidatesBeforeSpawn retains ownership through the handshake and uses the new placement.
func TestLoginRevalidatesBeforeSpawn(t *testing.T) {
	for _, mode := range []string{"same_world", "same_type_fallback", "other_type_fallback"} {
		t.Run(mode, func(t *testing.T) {
			var dimension world.Dimension = world.Overworld
			if mode == "other_type_fallback" {
				dimension = world.Nether
			}
			srv, a, c := admissionServer(t, dimension)
			srv.finaliseConn(t.Context(), c, admissionListener{})
			if a.releases != 0 || a.resolves != 1 {
				t.Fatal("handshake released the destination before spawn")
			}
			original := a.w
			fallback := original
			if mode != "same_world" {
				fallback = world.Config{Log: srv.conf.Log, Entities: entity.DefaultRegistry}.New()
				t.Cleanup(func() { _ = fallback.Close() })
				if err := original.Close(); err != nil {
					t.Fatal(err)
				}
			}
			a.w, a.data.Position = fallback, mgl64.Vec3{9, 80, 7}
			spawned := false
			for p := range srv.Accept() {
				if p.Tx().World() != fallback || p.Position() != a.data.Position || a.releases != 0 {
					t.Fatal("spawn used stale placement or released its reservation early")
				}
				if p.Data().Session.ChangingDimension() != (mode == "other_type_fallback") {
					t.Error("fallback did not use the session's dimension transition")
				}
				spawned = true
				_ = p.Close()
				break
			}
			if !spawned || a.resolves != 2 || a.releases != 1 {
				t.Fatalf("admission = spawned %t, %d resolutions, %d releases", spawned, a.resolves, a.releases)
			}
		})
	}
}

// TestClosedLoginDestinationReleasesAdmission tears down a failed spawn without an orphan session.
func TestClosedLoginDestinationReleasesAdmission(t *testing.T) {
	srv, a, c := admissionServer(t)
	srv.finaliseConn(t.Context(), c, admissionListener{})
	close(srv.incoming)
	if err := a.w.Close(); err != nil {
		t.Fatal(err)
	}
	for range srv.Accept() {
		t.Fatal("closed destination spawned a player")
	}
	if a.releases != 1 {
		t.Fatal("failed spawn leaked its admission")
	}
	srv.pwg.Wait()
	select {
	case <-c.closed:
	default:
		t.Fatal("failed spawn kept its connection")
	}
}

// A login is refused while another with the same UUID waits for admission, and admitted again once it is released.
// Releasing an old reservation leaves a newer one for the same UUID in place.
func TestLoginReservationRefusesDuplicateWhileAdmissionWaits(t *testing.T) {
	srv := &Server{p: make(map[uuid.UUID]*onlinePlayer)}
	id := uuid.New()
	first, ok := srv.reserveLogin(id)
	if !ok {
		t.Fatal("first login was refused")
	}
	if _, ok := srv.reserveLogin(id); ok {
		t.Fatal("second login with the same UUID was reserved while the first waits")
	}
	srv.releaseLogin(id, first)
	second, ok := srv.reserveLogin(id)
	if !ok {
		t.Fatal("login was refused after the failed admission released its UUID")
	}
	srv.releaseLogin(id, first)
	if _, ok := srv.reserveLogin(id); ok {
		t.Fatal("releasing the old reservation dropped the newer one")
	}
	srv.releaseLogin(id, second)
}

// A second login with the same UUID is refused while the first is handed off but not yet admitted.
func TestDuplicateLoginRefusedBeforeAdmission(t *testing.T) {
	srv, a, c := admissionServer(t)
	srv.incoming = make(chan incoming, 2)
	srv.finaliseConn(t.Context(), c, admissionListener{})
	if len(srv.incoming) != 1 {
		t.Fatal("first login was not handed off")
	}
	second := &admissionConn{id: c.id, closed: make(chan struct{})}
	t.Cleanup(func() { _ = second.Close() })
	srv.finaliseConn(t.Context(), second, admissionListener{})
	if len(srv.incoming) != 1 || a.releases != 1 {
		t.Fatalf("second login: %d pending players, %d releases; want 1 pending and its reservation released", len(srv.incoming), a.releases)
	}
}

// recordingAdmissionConn reports MovePlayer packets a session sends.
type recordingAdmissionConn struct {
	*admissionConn
	moves chan *packet.MovePlayer
}

// WritePacket reports MovePlayer packets and discards the rest.
func (c *recordingAdmissionConn) WritePacket(pk packet.Packet) error {
	if move, ok := pk.(*packet.MovePlayer); ok {
		select {
		case c.moves <- move:
		default:
		}
	}
	return nil
}

// A revalidated placement that only changes rotation still corrects the rotation sent in StartGame.
func TestLoginCorrectsRotationOnlyRevalidation(t *testing.T) {
	srv, a, base := admissionServer(t)
	c := &recordingAdmissionConn{admissionConn: base, moves: make(chan *packet.MovePlayer, 16)}
	srv.finaliseConn(t.Context(), c, admissionListener{})
	a.data.Rotation = cube.Rotation{90, 30}
	corrected := false
	for p := range srv.Accept() {
		timeout := time.After(10 * time.Second)
		for !corrected {
			select {
			case move := <-c.moves:
				corrected = move.Yaw == 90 && move.Pitch == 30
			case <-timeout:
				t.Error("no teleport corrected the rotation")
				corrected = true
			}
		}
		_ = p.Close()
		break
	}
}

// A join callback that panics still releases the login's admission.
func TestAcceptReleasesAdmissionWhenCallbackPanics(t *testing.T) {
	srv, a, c := admissionServer(t)
	srv.finaliseConn(t.Context(), c, admissionListener{})
	func() {
		defer func() { _ = recover() }()
		for p := range srv.Accept() {
			_ = p.Close()
			panic("callback")
		}
	}()
	if a.releases != 1 {
		t.Fatalf("admission released %d times, want 1", a.releases)
	}
}

// A login admitted after shutdown's disconnect pass is disconnected instead of joining.
func TestLoginAdmittedDuringShutdownIsDisconnected(t *testing.T) {
	srv, _, c := admissionServer(t)
	srv.finaliseConn(t.Context(), c, admissionListener{})
	srv.closing.Store(true)
	joined := false
	done := make(chan struct{})
	go func() {
		defer close(done)
		for p := range srv.Accept() {
			joined = true
			_ = p.Close()
			break
		}
	}()
	select {
	case <-c.closed:
	case <-time.After(10 * time.Second):
		t.Fatal("the login admitted during shutdown was not disconnected")
	}
	close(srv.incoming)
	<-done
	if joined {
		t.Fatal("a login admitted during shutdown joined")
	}
}
