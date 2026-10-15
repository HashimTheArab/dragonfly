package session

import (
	"bytes"
	"log/slog"
	"reflect"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// subChunkCacheConn supplies only the connection fact used to build an entry.
type subChunkCacheConn struct {
	Conn
	cacheEnabled bool
}

// ClientCacheEnabled reports whether this test connection accepts terrain blobs.
func (c subChunkCacheConn) ClientCacheEnabled() bool {
	return c.cacheEnabled
}

// subChunkNBTBlock retains its NBT so tests can detect unwanted encoding or edits.
type subChunkNBTBlock struct {
	world.Block
	data        map[string]any
	encodeCalls int
}

// EncodeNBT exposes the test actor's metadata and records each encoding attempt.
func (b *subChunkNBTBlock) EncodeNBT() map[string]any {
	b.encodeCalls++
	return b.data
}

// DecodeNBT satisfies the block actor interface without changing this fixture.
func (b *subChunkNBTBlock) DecodeNBT(map[string]any) any {
	return b
}

// TestSessionSubChunkEntryCacheContract checks the native payload and cache boundary.
func TestSessionSubChunkEntryCacheContract(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	for _, tt := range []struct {
		name         string
		cacheEnabled bool
		pending      int
		wantCached   bool
	}{
		{name: "noncached populated"},
		{name: "cached accepted at pending limit", cacheEnabled: true, pending: 4096, wantCached: true},
		{name: "cached rejected above pending limit", cacheEnabled: true, pending: 4097},
	} {
		t.Run(tt.name, func(t *testing.T) {
			col, index, stone, actor, otherActor := nativeSubChunkColumn()
			terrain := chunk.EncodeSubChunk(col.Chunk, chunk.NetworkEncoding, int(index))
			wantHash := xxhash.Sum64(terrain)
			s := newSubChunkTestSession(tt.cacheEnabled, tt.pending)
			if _, exists := s.blobs[wantHash]; exists {
				t.Fatal("fixture terrain hash collides with a pre-existing blob")
			}
			transaction := map[uint64]struct{}{0: {}}
			offset := protocol.SubChunkOffset{-2, 3, 1}

			entry := s.subChunkEntry(offset, index, col, chunk.NewSubChunkHeightMaps(col.Chunk), transaction)
			if entry.Result != protocol.SubChunkResultSuccess {
				t.Fatalf("result = %d, want Success", entry.Result)
			}
			assertNativeSubChunkSurface(t, entry, offset)
			raw, present := entry.RawPayload.Value()
			if !present {
				t.Fatal("raw payload is absent")
			}
			hash, cached := entry.BlobHash.Value()
			if cached != tt.wantCached {
				t.Fatalf("blob hash present = %v, want %v", cached, tt.wantCached)
			}
			if tt.wantCached {
				if hash != wantHash {
					t.Fatalf("blob hash = %x, want terrain xxhash %x", hash, wantHash)
				}
				blob, tracked := s.blobs[hash]
				if !tracked || !bytes.Equal(blob, terrain) {
					t.Fatal("session did not retain the terrain blob under its hash")
				}
				if tail := decodeNativeSubChunkTerrain(t, blob, index, stone); len(tail) != 0 {
					t.Fatalf("terrain blob contains %d trailing bytes, want no block actor data", len(tail))
				}
				if len(s.blobs) != tt.pending+1 {
					t.Fatalf("pending blobs = %d, want %d", len(s.blobs), tt.pending+1)
				}
				if _, tracked := transaction[hash]; !tracked || len(transaction) != 2 {
					t.Fatalf("request transaction = %v, want original entry and terrain hash", transaction)
				}
				assertNativeSubChunkActor(t, raw)
			} else {
				if !bytes.HasPrefix(raw, terrain) {
					t.Fatal("uncached payload is missing the complete terrain prefix")
				}
				assertNativeSubChunkActor(t, decodeNativeSubChunkTerrain(t, raw, index, stone))
				if len(s.blobs) != tt.pending {
					t.Fatalf("pending blobs = %d, want unchanged %d", len(s.blobs), tt.pending)
				}
				if _, tracked := s.blobs[wantHash]; tracked {
					t.Fatal("uncached terrain was added to the session blob map")
				}
				if len(transaction) != 1 {
					t.Fatalf("uncached entry changed the request transaction: %v", transaction)
				}
			}
			if _, retained := transaction[0]; !retained {
				t.Fatal("entry removed the request transaction's existing hash")
			}
			assertNativePendingBlobs(t, s, tt.pending)
			if len(s.openChunkTransactions) != 0 {
				t.Fatal("entry opened a session transaction before the response was assembled")
			}
			if actor.encodeCalls != 1 || otherActor.encodeCalls != 0 {
				t.Fatalf("actor encode calls = (%d, %d), want (1, 0)", actor.encodeCalls, otherActor.encodeCalls)
			}
			if actor.data["x"] != int32(-999) || actor.data["y"] != int32(-999) || actor.data["z"] != int32(-999) {
				t.Fatalf("entry changed actor-owned metadata: %v", actor.data)
			}
		})
	}
}

// TestSessionSubChunkEntryCachedAllAir preserves the native early-empty contract.
func TestSessionSubChunkEntryCachedAllAir(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	col, _, _, actor, _ := nativeSubChunkColumn()
	index := col.SubIndex(80)
	col.BlockEntities = map[cube.Pos]world.Block{{36, 86, -43}: actor}
	s := newSubChunkTestSession(true, 4097)
	transaction := map[uint64]struct{}{0: {}}
	offset := protocol.SubChunkOffset{1, -2, -3}

	entry := s.subChunkEntry(offset, index, col, chunk.NewSubChunkHeightMaps(col.Chunk), transaction)
	if entry.Result != protocol.SubChunkResultSuccessAllAir || entry.Offset != offset {
		t.Fatalf("entry = (result %d, offset %v), want (SuccessAllAir, %v)", entry.Result, entry.Offset, offset)
	}
	if entry.HeightMapType != protocol.HeightMapDataTooLow || entry.RenderHeightMapType != protocol.HeightMapDataTooLow {
		t.Fatalf("height map types = (%d, %d), want (TooLow, TooLow)", entry.HeightMapType, entry.RenderHeightMapType)
	}
	if _, present := entry.HeightMapData.Value(); present {
		t.Error("all-air entry above the surface carries height samples")
	}
	if _, present := entry.RenderHeightMapData.Value(); present {
		t.Error("all-air entry above the surface carries render height samples")
	}
	if _, present := entry.RawPayload.Value(); present {
		t.Error("native all-air entry carries a payload")
	}
	if _, present := entry.BlobHash.Value(); present {
		t.Error("native all-air entry carries a terrain blob hash")
	}
	if actor.encodeCalls != 0 {
		t.Fatalf("all-air actor encoded %d times, want native early return", actor.encodeCalls)
	}
	if len(s.blobs) != 4097 || len(transaction) != 1 {
		t.Fatalf("all-air entry changed cache state: blobs=%d transaction=%v", len(s.blobs), transaction)
	}
	if _, retained := transaction[0]; !retained {
		t.Fatal("all-air entry removed an existing request hash")
	}
	assertNativePendingBlobs(t, s, 4097)
	if len(s.openChunkTransactions) != 0 {
		t.Fatal("all-air entry opened a session transaction")
	}
}

// newSubChunkTestSession initializes only the native entry builder's dependencies.
func newSubChunkTestSession(cacheEnabled bool, pending int) *Session {
	s := &Session{
		conn:  subChunkCacheConn{cacheEnabled: cacheEnabled},
		conf:  Config{Log: slog.New(slog.DiscardHandler)},
		blobs: make(map[uint64][]byte, pending),
	}
	for i := range pending {
		s.blobs[uint64(i)] = []byte{0xa5}
	}
	return s
}

// nativeSubChunkColumn supplies a floor, one raised block, and two actor positions.
func nativeSubChunkColumn() (*world.Column, int16, uint32, *subChunkNBTBlock, *subChunkNBTBlock) {
	c := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	stone := world.DefaultBlockRegistry.BlockRuntimeID(block.Stone{})
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			c.SetBlock(x, 64, z, 0, stone)
		}
	}
	c.SetBlock(4, 70, 5, 0, stone)
	actor := &subChunkNBTBlock{data: map[string]any{
		"id": "Chest", "CustomName": "native entry",
		"x": int32(-999), "y": int32(-999), "z": int32(-999),
	}}
	otherActor := &subChunkNBTBlock{data: map[string]any{"id": "Chest", "CustomName": "other sub-chunk"}}
	return &world.Column{
		Chunk: c,
		BlockEntities: map[cube.Pos]world.Block{
			{36, 70, -43}: actor,
			{35, 80, -42}: otherActor,
		},
	}, c.SubIndex(64), stone, actor, otherActor
}

// assertNativeSubChunkSurface checks explicit surface heights in both client maps.
func assertNativeSubChunkSurface(t *testing.T, entry protocol.SubChunkEntry, offset protocol.SubChunkOffset) {
	t.Helper()
	if entry.Offset != offset {
		t.Fatalf("offset = %v, want %v", entry.Offset, offset)
	}
	if entry.HeightMapType != protocol.HeightMapDataHasData || entry.RenderHeightMapType != protocol.HeightMapDataHasData {
		t.Fatalf("height map types = (%d, %d), want (HasData, HasData)", entry.HeightMapType, entry.RenderHeightMapType)
	}
	heights, present := entry.HeightMapData.Value()
	renderHeights, renderPresent := entry.RenderHeightMapData.Value()
	if !present || !renderPresent {
		t.Fatalf("height map presence = (%v, %v), want both", present, renderPresent)
	}
	for z := range heights {
		for x := range heights[z] {
			want := int8(1)
			if x == 4 && z == 5 {
				want = 7
			}
			if heights[z][x] != want || renderHeights[z][x] != want {
				t.Fatalf("heights[%d][%d] = (%d, %d), want (%d, %d)", z, x, heights[z][x], renderHeights[z][x], want, want)
			}
		}
	}
}

// decodeNativeSubChunkTerrain checks every decoded block and returns the actor tail.
func decodeNativeSubChunkTerrain(t *testing.T, raw []byte, index int16, stone uint32) []byte {
	t.Helper()
	buf := bytes.NewBuffer(raw)
	c := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	var decodedIndex byte
	sub, err := chunk.DecodeSubChunk(buf, c, &decodedIndex, chunk.NetworkEncoding)
	if err != nil {
		t.Fatalf("decode terrain: %v", err)
	}
	if int16(decodedIndex) != index {
		t.Fatalf("decoded index = %d, want %d", decodedIndex, index)
	}
	for x := byte(0); x < 16; x++ {
		for y := byte(0); y < 16; y++ {
			for z := byte(0); z < 16; z++ {
				want := world.DefaultBlockRegistry.AirRuntimeID()
				if y == 0 || (x == 4 && y == 6 && z == 5) {
					want = stone
				}
				if got := sub.Block(x, y, z, 0); got != want {
					t.Fatalf("decoded terrain at (%d, %d, %d) = %d, want %d", x, y, z, got, want)
				}
			}
		}
	}
	return buf.Bytes()
}

// assertNativeSubChunkActor decodes the exact positional metadata visible to clients.
func assertNativeSubChunkActor(t *testing.T, raw []byte) {
	t.Helper()
	actors, err := chunk.DecodeBlockNBTs(bytes.NewBuffer(raw))
	if err != nil {
		t.Fatalf("decode actor data: %v", err)
	}
	want := []map[string]any{{
		"id": "Chest", "CustomName": "native entry",
		"x": int32(36), "y": int32(70), "z": int32(-43),
	}}
	if !reflect.DeepEqual(actors, want) {
		t.Fatalf("decoded actors = %#v, want %#v", actors, want)
	}
}

// assertNativePendingBlobs checks that adding or rejecting an entry preserves old blobs.
func assertNativePendingBlobs(t *testing.T, s *Session, pending int) {
	t.Helper()
	for i := range pending {
		if blob, present := s.blobs[uint64(i)]; !present || !bytes.Equal(blob, []byte{0xa5}) {
			t.Fatalf("pre-existing blob %d changed: present=%v data=%x", i, present, blob)
		}
	}
}
