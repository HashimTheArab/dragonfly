package session

import (
	"io"
	"log/slog"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestMaxPendingChunkBlobsDefaultsAndAllowsOverride(t *testing.T) {
	if got := (Config{}).maxPendingChunkBlobs(); got != 4096 {
		t.Fatalf("default max pending chunk blobs = %d, want 4096", got)
	}
	if got := (Config{MaxPendingChunkBlobs: 32}).maxPendingChunkBlobs(); got != 32 {
		t.Fatalf("configured max pending chunk blobs = %d, want 32", got)
	}
}

func TestTrackBlobHonoursConfiguredLimit(t *testing.T) {
	s := &Session{
		conf: Config{
			Log:                  slog.New(slog.NewTextHandler(io.Discard, nil)),
			MaxPendingChunkBlobs: 2,
		},
		blobs: map[uint64][]byte{},
	}
	if !s.trackBlob(1, []byte{1}) || !s.trackBlob(2, []byte{2}) {
		t.Fatal("trackBlob rejected a blob below the configured limit")
	}
	if !s.trackBlob(2, []byte{2}) {
		t.Fatal("trackBlob rejected an already tracked blob at the configured limit")
	}
	if s.trackBlob(3, []byte{3}) {
		t.Fatal("trackBlob accepted a blob above the configured limit")
	}
}

func TestTrackBlobsRejectsWholeBatchAboveConfiguredLimit(t *testing.T) {
	s := &Session{
		conf: Config{
			Log:                  slog.New(slog.NewTextHandler(io.Discard, nil)),
			MaxPendingChunkBlobs: 2,
		},
		blobs: map[uint64][]byte{1: {1}},
	}
	if s.trackBlobs([]uint64{2, 3}, [][]byte{{2}, {3}}) {
		t.Fatal("trackBlobs accepted a batch that exceeded the configured limit")
	}
	if len(s.blobs) != 1 {
		t.Fatalf("trackBlobs partially stored a rejected batch: got %d blobs, want 1", len(s.blobs))
	}
	if !s.trackBlobs([]uint64{1, 2}, [][]byte{{1}, {2}}) {
		t.Fatal("trackBlobs counted an existing hash against the configured limit")
	}
}

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
