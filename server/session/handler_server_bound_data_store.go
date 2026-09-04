package session

import (
	"github.com/df-mc/dragonfly/server/player/ddui"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type ServerBoundDataStoreHandler struct {
	h *ddui.Controller
}

func (d *ServerBoundDataStoreHandler) Handle(p packet.Packet, s *Session, _ *world.Tx, _ Controllable) error {
	d.h.HandleDataStore(p.(*packet.ServerBoundDataStore))
	return nil
}
