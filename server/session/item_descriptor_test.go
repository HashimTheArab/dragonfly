package session

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type descriptorTestConn struct {
	shieldID int32
}

func (c descriptorTestConn) Close() error                                               { return nil }
func (c descriptorTestConn) IdentityData() login.IdentityData                           { return login.IdentityData{} }
func (c descriptorTestConn) ClientData() login.ClientData                               { return login.ClientData{} }
func (c descriptorTestConn) ClientCacheEnabled() bool                                   { return false }
func (c descriptorTestConn) ChunkRadius() int                                           { return 0 }
func (c descriptorTestConn) Latency() time.Duration                                     { return 0 }
func (c descriptorTestConn) Flush() error                                               { return nil }
func (c descriptorTestConn) RemoteAddr() net.Addr                                       { return nil }
func (c descriptorTestConn) ReadPacket() (packet.Packet, error)                         { return nil, io.EOF }
func (c descriptorTestConn) WritePacket(packet.Packet) error                            { return nil }
func (c descriptorTestConn) StartGameContext(context.Context, minecraft.GameData) error { return nil }
func (c descriptorTestConn) ShieldID() int32                                            { return c.shieldID }

func TestSessionItemDescriptorHelpersRoundTripEmptyStack(t *testing.T) {
	s := &Session{
		conn: descriptorTestConn{shieldID: 512},
		br:   world.DefaultBlockRegistry,
	}

	desc := s.descriptorFromItem(item.Stack{})
	if desc != (protocol.NetworkItemStackDescriptor{}) {
		t.Fatalf("empty stack descriptor = %#v, want zero descriptor", desc)
	}

	out := s.itemFromDescriptor(desc)
	if !out.Empty() {
		t.Fatalf("empty descriptor decoded to %#v, want empty stack", out)
	}
}
