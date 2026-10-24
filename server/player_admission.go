package server

import (
	"context"
	"errors"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// admitPlayer revalidates a pending login immediately before creating and spawning its player.
// A destination that closes outside the provider's reservation is resolved again.
func (srv *Server) admitPlayer(inc incoming, yield func(*player.Player) bool) (bool, error) {
	conf, w := inc.conf, inc.w
	if inc.admission != nil {
		conf, w = inc.admission.Resolve()
	}
	for {
		ret, err := world.Call(context.Background(), w, func(tx *world.Tx) (bool, error) {
			id := uuid.MustParse(inc.conn.IdentityData().Identity)
			s, online := srv.createPlayer(id, inc.conn, conf, w)
			srv.pmu.Lock()
			srv.p[id] = online
			srv.pmu.Unlock()
			p := tx.AddEntity(online.handle).(*player.Player)
			// StartGame described the original placement. Correct a fallback before spawn.
			if w != inc.w {
				if w.Dimension() != inc.w.Dimension() {
					dim, _ := world.DimensionID(w.Dimension())
					_ = inc.conn.WritePacket(&packet.ChangeDimension{Dimension: int32(dim), Position: vec64To32(conf.Position.Add(mgl64.Vec3{0, player.Type.NetworkOffset()})), Respawn: false})
				}
				s.ViewEntityTeleport(p, conf.Position)
			}
			s.Spawn(p, tx)
			return !yield(p), nil
		})
		if !errors.Is(err, world.ErrWorldClosed) || inc.admission == nil {
			return ret, err
		}
		nextConf, nextWorld := inc.admission.Resolve()
		if nextWorld == w {
			return ret, err
		}
		conf, w = nextConf, nextWorld
	}
}
