package server

import (
	"context"
	"errors"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
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
			s, online := srv.createPlayer(inc, conf, w)
			srv.pmu.Lock()
			srv.p[online.handle.UUID()] = online
			srv.pmu.Unlock()
			p := tx.AddEntity(online.handle).(*player.Player)
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
