package session

import (
	"github.com/df-mc/dragonfly/server/player/ddui"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// DataDrivenScreenClosedHandler handles the ServerBoundDataDrivenScreenClosed packet.
type DataDrivenScreenClosedHandler struct {
	h *ddui.Controller
}

func (d *DataDrivenScreenClosedHandler) Handle(p packet.Packet, s *Session, _ *world.Tx, _ Controllable) error {
	d.h.HandleScreenClosed(p.(*packet.ServerBoundDataDrivenScreenClosed))
	return nil
}
