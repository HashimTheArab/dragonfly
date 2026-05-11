package session

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type nilNBTBlock struct {
	block.Note
}

func (nilNBTBlock) EncodeNBT() map[string]any { return nil }

func TestSendFullNetworkChunkHandlesNilBlockEntityNBT(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	s := &Session{
		packets:         make(chan packet.Packet, 1),
		closeBackground: make(chan struct{}),
	}
	c := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("sendFullNetworkChunk panicked: %v", r)
		}
	}()
	s.sendFullNetworkChunk(world.ChunkPos{}, world.Overworld, c, map[cube.Pos]world.Block{
		{}: nilNBTBlock{},
	})
}
