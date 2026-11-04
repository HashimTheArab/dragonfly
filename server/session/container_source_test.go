package session

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestSession_InventorySourceValidatesOpenedBlock(t *testing.T) {
	w := world.Config{Synchronous: true}.New()
	t.Cleanup(func() { _ = w.Close() })
	s := &Session{}
	pos := cube.Pos{0, 64, 0}
	opened := inventory.New(27, nil)
	s.openedPos.Store(&pos)
	s.openedWindow.Store(opened)
	if _, ok := s.invByID(protocol.ContainerShulkerBox, nil); ok {
		t.Fatal("source was accepted while no container was open")
	}
	s.containerOpened.Store(true)
	w.Do(func(tx *world.Tx) {
		for _, test := range []struct {
			name  string
			block world.Block
			id    int32
			want  bool
		}{
			{name: "shulker source", block: block.NewShulkerBox(), id: protocol.ContainerShulkerBox, want: true},
			{name: "barrel source on shulker", block: block.NewShulkerBox(), id: protocol.ContainerBarrel},
			{name: "barrel source", block: block.NewBarrel(), id: protocol.ContainerBarrel, want: true},
			{name: "shulker source after replacement", block: block.NewBarrel(), id: protocol.ContainerShulkerBox},
			{name: "generic source", block: block.NewShulkerBox(), id: protocol.ContainerLevelEntity, want: true},
			{name: "specialised source on chest", block: block.NewChest(), id: protocol.ContainerShulkerBox},
			{name: "specialised source on workstation", block: block.Furnace{}, id: protocol.ContainerBarrel},
		} {
			t.Run(test.name, func(t *testing.T) {
				tx.SetBlock(pos, test.block, nil)
				got, ok := s.invByID(test.id, tx)
				if ok != test.want || (ok && got != opened) {
					t.Fatalf("invByID(%d) = %p, %t; want opened=%p, %t", test.id, got, ok, opened, test.want)
				}
			})
		}
	})
}
