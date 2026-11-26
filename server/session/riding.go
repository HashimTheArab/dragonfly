package session

import (
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// viewRidingLinks restores links after both actors have been introduced to this viewer.
func (s *Session) viewRidingLinks(e world.Entity) {
	if vehicle, seat, ok := world.Riding(e); ok {
		s.viewEntityLink(vehicle, world.EntityLinkAction{Rider: e.H(), Seat: seat})
	}
	for _, link := range world.RiderLinks(e) {
		s.viewEntityLink(e.H(), world.EntityLinkAction{Rider: link.Rider, Seat: link.Seat})
	}
}

// viewEntityLink encodes vehicle first and rider second, using this viewer's actor IDs.
func (s *Session) viewEntityLink(vehicle *world.EntityHandle, action world.EntityLinkAction) {
	if action.Remove && action.Rider == s.ent {
		// The server-owned seat position supersedes any pending teleport destination.
		s.teleportPos.Store(nil)
	}
	s.entityMutex.RLock()
	vehicleID, vehicleKnown := s.entityRuntimeIDs[vehicle]
	riderID, riderKnown := s.entityRuntimeIDs[action.Rider]
	s.entityMutex.RUnlock()
	if !vehicleKnown || !riderKnown {
		return
	}
	kind := byte(protocol.EntityLinkPassenger)
	if action.Seat.Controlling {
		kind = protocol.EntityLinkRider
	}
	if action.Remove {
		kind = protocol.EntityLinkRemove
	}
	s.writePacket(&packet.SetActorLink{EntityLink: protocol.EntityLink{
		RiddenEntityUniqueID: int64(vehicleID), RiderEntityUniqueID: int64(riderID),
		Type: kind, Immediate: action.Remove, RiderInitiated: true,
	}})
}
