package chunk_test

import (
	"bytes"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestSubChunkHeightMaps_EntryEmpty(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	for _, storage := range []bool{false, true} {
		t.Run(map[bool]string{false: "no storage", true: "single air storage"}[storage], func(t *testing.T) {
			c := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
			index := c.SubIndex(64)
			if storage {
				c.SetBlock(0, 64, 0, 0, world.DefaultBlockRegistry.AirRuntimeID())
			}
			offset := protocol.SubChunkOffset{-7, 3, 6}
			entry := chunk.NewSubChunkHeightMaps(c).Entry(index, offset, []byte("terrain"), []byte("actors"))
			if entry.Result != protocol.SubChunkResultSuccessAllAir || entry.Offset != offset {
				t.Fatalf("empty entry result/offset = %d/%v", entry.Result, entry.Offset)
			}
			if _, present := entry.RawPayload.Value(); present {
				t.Fatal("empty entry carried a payload")
			}
			if _, present := entry.BlobHash.Value(); present {
				t.Fatal("non-cache empty entry carried a blob hash")
			}
			if entry.HeightMapType != protocol.HeightMapDataTooLow || entry.RenderHeightMapType != protocol.HeightMapDataTooLow {
				t.Fatalf("empty entry height types = %d/%d, want TooLow", entry.HeightMapType, entry.RenderHeightMapType)
			}
			if _, present := entry.HeightMapData.Value(); present {
				t.Fatal("empty entry carried heights for a surface below the band")
			}
			if _, present := entry.RenderHeightMapData.Value(); present {
				t.Fatal("empty entry carried render heights for a surface below the band")
			}
		})
	}
}

func TestSubChunkHeightMaps_EntryNativeHeights(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	c := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	stone := world.DefaultBlockRegistry.BlockRuntimeID(block.Stone{})
	for x := uint8(0); x < 16; x++ {
		for z := uint8(0); z < 16; z++ {
			c.SetBlock(x, 64, z, 0, stone)
		}
	}
	index := c.SubIndex(64)
	maps := chunk.NewSubChunkHeightMaps(c)
	for _, tc := range []struct {
		name    string
		index   int16
		mapType byte
		result  byte
	}{
		{"below", index - 1, protocol.HeightMapDataTooHigh, protocol.SubChunkResultSuccessAllAir},
		{"surface", index, protocol.HeightMapDataHasData, protocol.SubChunkResultSuccess},
		{"above", index + 1, protocol.HeightMapDataTooLow, protocol.SubChunkResultSuccessAllAir},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := maps.Entry(tc.index, protocol.SubChunkOffset{}, []byte{1}, nil)
			if entry.Result != tc.result || entry.HeightMapType != tc.mapType || entry.RenderHeightMapType != tc.mapType {
				t.Fatalf("entry result/types = %d/%d/%d, want %d/%d/%d", entry.Result, entry.HeightMapType, entry.RenderHeightMapType, tc.result, tc.mapType, tc.mapType)
			}
			for _, heights := range []protocol.Optional[protocol.HeightMap]{entry.HeightMapData, entry.RenderHeightMapData} {
				heightMap, present := heights.Value()
				if present != (tc.mapType == protocol.HeightMapDataHasData) {
					t.Fatalf("height map present = %t for type %d", present, tc.mapType)
				}
				if present {
					for z, row := range heightMap {
						for x, height := range row {
							if height != 1 {
								t.Fatalf("height[%d][%d] = %d, want first free Y 65 relative to band base 64", z, x, height)
							}
						}
					}
				}
			}
		})
	}

	c.SetBlock(2, 96, 3, 0, stone)
	c.SetBlock(4, 70, 5, 0, stone)
	c.SetBlock(10, 80, 12, 0, stone)
	entry := chunk.NewSubChunkHeightMaps(c).Entry(c.SubIndex(80), protocol.SubChunkOffset{}, []byte{1}, nil)
	if entry.Result != protocol.SubChunkResultSuccess || entry.HeightMapType != protocol.HeightMapDataHasData || entry.RenderHeightMapType != protocol.HeightMapDataHasData {
		t.Fatalf("mixed entry result/types = %d/%d/%d", entry.Result, entry.HeightMapType, entry.RenderHeightMapType)
	}
	for _, heights := range []protocol.Optional[protocol.HeightMap]{entry.HeightMapData, entry.RenderHeightMapData} {
		heightMap, present := heights.Value()
		if !present {
			t.Fatal("mixed height map absent")
		}
		for z, row := range heightMap {
			for x, height := range row {
				want := int8(-1)
				if x == 2 && z == 3 {
					want = 16
				}
				if x == 10 && z == 12 {
					want = 1
				}
				if height != want {
					t.Fatalf("mixed height[%d][%d] = %d, want %d", z, x, height, want)
				}
			}
		}
	}
}

func TestSubChunkHeightMaps_EntryOwnsPayload(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	c := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	c.SetBlock(0, 64, 0, 0, world.DefaultBlockRegistry.BlockRuntimeID(block.Stone{}))
	terrain := []byte{1, 2, 3, 4}[:2]
	actors := []byte{5, 6}
	entry := chunk.NewSubChunkHeightMaps(c).Entry(c.SubIndex(64), protocol.SubChunkOffset{}, terrain, actors)
	raw, present := entry.RawPayload.Value()
	if !present || !bytes.Equal(raw, []byte{1, 2, 5, 6}) {
		t.Fatalf("payload = %v, present = %t", raw, present)
	}
	if !bytes.Equal(terrain[:cap(terrain)], []byte{1, 2, 3, 4}) || !bytes.Equal(actors, []byte{5, 6}) {
		t.Fatal("entry assembly mutated its input buffers")
	}
	terrain[0], actors[0] = 8, 9
	if !bytes.Equal(raw, []byte{1, 2, 5, 6}) {
		t.Fatal("input buffer reuse changed the prepared payload")
	}
	raw[1], raw[3] = 7, 7
	if terrain[1] != 2 || actors[1] != 6 {
		t.Fatal("payload mutation reached the input buffers")
	}
	if _, present := entry.BlobHash.Value(); present {
		t.Fatal("non-cache payload carried a blob hash")
	}
}

func TestSubChunkHeightMaps_EntryPreservesSecondaryOnlyTerrain(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	registry := world.DefaultBlockRegistry
	c := chunk.New(registry, world.Overworld.Range())
	water := registry.BlockRuntimeID(block.Water{Still: true, Depth: 8})
	c.SetBlock(2, 66, 2, 1, water)
	index := c.SubIndex(64)
	terrain := chunk.EncodeSubChunk(c, chunk.NetworkEncoding, int(index))
	entry := chunk.NewSubChunkHeightMaps(c).Entry(index, protocol.SubChunkOffset{}, terrain, nil)
	raw, present := entry.RawPayload.Value()
	if entry.Result != protocol.SubChunkResultSuccess || !present || !bytes.Equal(raw, terrain) {
		t.Fatal("secondary-only terrain was collapsed to an all-air response")
	}
	buf := bytes.NewBuffer(raw)
	subIndex := byte(index)
	sub, err := chunk.DecodeSubChunk(buf, chunk.New(registry, world.Overworld.Range()), &subIndex, chunk.NetworkEncoding)
	if err != nil {
		t.Fatal(err)
	}
	if sub.Block(2, 2, 2, 0) != registry.AirRuntimeID() || sub.Block(2, 2, 2, 1) != water || buf.Len() != 0 {
		t.Fatal("secondary-only response changed either layer or appended data")
	}
}

func TestSubChunkHeightMaps_EntryWireTerrainAndBlockEntities(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	t.Parallel()

	for _, hashes := range []bool{false, true} {
		t.Run(map[bool]string{false: "runtime IDs", true: "network hashes"}[hashes], func(t *testing.T) {
			registry := world.DefaultBlockRegistry
			c := chunk.New(registry, world.Overworld.Range())
			stone := registry.BlockRuntimeID(block.Stone{})
			water := registry.BlockRuntimeID(block.Water{Still: true, Depth: 8})
			c.SetBlock(3, 64, 5, 0, stone)
			c.SetBlock(2, 66, 2, 1, water)
			index := c.SubIndex(64)
			terrain := chunk.EncodeSubChunk(c, chunk.NetworkEncoding, int(index))
			if hashes {
				terrain = chunk.EncodeSubChunkWithBlockNetworkHashes(c, int(index))
			}
			actors, err := chunk.EncodeBlockEntities([]chunk.BlockEntity{{
				Pos: cube.Pos{1, 65, 1},
				Data: map[string]any{
					"id": "Chest", "pairx": int32(9), "pairz": int32(11),
					"custom": map[string]any{"kept": int32(17)},
				},
			}})
			if err != nil {
				t.Fatal(err)
			}
			offset := protocol.SubChunkOffset{-2, 0, 1}
			entry := chunk.NewSubChunkHeightMaps(c).Entry(index, offset, terrain, actors)
			outgoing := &packet.SubChunk{
				Dimension: packet.DimensionOverworld, Position: protocol.SubChunkPos{5, 4, -7},
				SubChunkEntries: []protocol.SubChunkEntry{entry},
			}
			var wire bytes.Buffer
			outgoing.Marshal(protocol.NewWriter(&wire, 0))
			incoming := &packet.SubChunk{}
			reader := bytes.NewBuffer(wire.Bytes())
			incoming.Marshal(protocol.NewReader(reader, 0, true))
			if reader.Len() != 0 || incoming.CacheEnabled || incoming.Dimension != packet.DimensionOverworld || incoming.Position != outgoing.Position || len(incoming.SubChunkEntries) != 1 {
				t.Fatalf("response changed on wire: %+v, trailing bytes = %d", incoming, reader.Len())
			}
			got := incoming.SubChunkEntries[0]
			if got.Result != protocol.SubChunkResultSuccess || got.Offset != offset {
				t.Fatalf("wire result/offset = %d/%v", got.Result, got.Offset)
			}
			if got.HeightMapType != protocol.HeightMapDataHasData || got.RenderHeightMapType != protocol.HeightMapDataHasData {
				t.Fatalf("wire height types = %d/%d, want both HasData", got.HeightMapType, got.RenderHeightMapType)
			}
			for _, heights := range []protocol.Optional[protocol.HeightMap]{got.HeightMapData, got.RenderHeightMapData} {
				heightMap, present := heights.Value()
				if !present || heightMap[5][3] != 1 {
					t.Fatal("wire entry lost the surface's first free Y 65 relative to band base 64")
				}
			}
			raw, present := got.RawPayload.Value()
			if !present || !bytes.HasPrefix(raw, terrain) || !bytes.Equal(raw[len(terrain):], actors) {
				t.Fatal("wire payload changed encoded terrain or trailing actor bytes")
			}
			buf := bytes.NewBuffer(raw)
			subIndex := byte(index)
			sub, err := chunk.DecodeSubChunk(buf, chunk.New(registry, world.Overworld.Range()), &subIndex, chunk.NetworkEncoding)
			if err != nil {
				t.Fatal(err)
			}
			if subIndex != byte(index) {
				t.Fatalf("decoded band index = %d, want %d", subIndex, index)
			}
			if hashes {
				stoneHash, ok := registry.RuntimeIDToHash(stone)
				if !ok || sub.Block(3, 0, 5, 0) != stoneHash {
					t.Fatal("entry lost negotiated network hash encoding")
				}
				sub.ConvertBlockNetworkHashesToRuntimeIDs(registry)
			}
			if sub.Block(3, 0, 5, 0) != stone || sub.Block(2, 2, 2, 1) != water || c.Block(3, 64, 5, 0) != stone {
				t.Fatal("entry changed native terrain, secondary layer, or source chunk")
			}
			decoded, err := chunk.DecodeBlockNBTs(buf)
			if err != nil {
				t.Fatal(err)
			}
			if buf.Len() != 0 || len(decoded) != 1 {
				t.Fatalf("decoded actors = %d, trailing bytes = %d", len(decoded), buf.Len())
			}
			actor := decoded[0]
			custom, ok := actor["custom"].(map[string]any)
			if !ok || custom["kept"] != int32(17) || actor["id"] != "Chest" || actor["x"] != int32(1) || actor["y"] != int32(65) || actor["z"] != int32(1) || actor["pairx"] != int32(9) || actor["pairz"] != int32(11) {
				t.Fatalf("native actor metadata changed: %v", actor)
			}
		})
	}
}
