package chunk_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
)

// TestBlockEntitiesSnapshotOrdersAndDetaches verifies that every mutable NBT
// container and the outer slice can be changed without altering source terrain.
func TestBlockEntitiesSnapshotOrdersAndDetaches(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	ch := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	data := map[string]any{
		"id":         "Chest",
		"compound":   map[string]any{"text": "original"},
		"list":       []any{map[string]any{"value": int32(1)}},
		"typed_list": []map[string]any{{"value": int32(2)}},
		"bytes":      []byte{3}, "ints": []int32{4}, "longs": []int64{5},
		"array": [1][]int32{{6}},
	}
	positions := []cube.Pos{{34, 64, 49}, {32, 63, 49}, {33, 64, 48}, {32, 64, 48}}
	for _, pos := range positions {
		ch.SetBlockEntityData(pos, data)
	}
	snapshot := ch.BlockEntities()
	wantPositions := []cube.Pos{{32, 63, 49}, {32, 64, 48}, {33, 64, 48}, {34, 64, 49}}
	for i, entry := range snapshot {
		if entry.Pos != wantPositions[i] {
			t.Fatalf("position %d = %v, want %v", i, entry.Pos, wantPositions[i])
		}
		if !reflect.DeepEqual(entry.Data, data) {
			t.Fatalf("snapshot changed NBT: %#v", entry.Data)
		}
	}
	first := snapshot[0].Data
	first["compound"].(map[string]any)["text"] = "changed"
	first["list"].([]any)[0].(map[string]any)["value"] = int32(10)
	first["typed_list"].([]map[string]any)[0]["value"] = int32(20)
	first["bytes"].([]byte)[0] = 30
	first["ints"].([]int32)[0] = 40
	first["longs"].([]int64)[0] = 50
	first["array"].([1][]int32)[0][0] = 60
	snapshot[0].Pos = cube.Pos{}
	fresh := ch.BlockEntities()
	for i, entry := range fresh {
		if entry.Pos != wantPositions[i] || !reflect.DeepEqual(entry.Data, data) {
			t.Fatalf("snapshot aliases source at %d: %#v", i, entry)
		}
	}
	if !reflect.DeepEqual(snapshot[1].Data, data) {
		t.Fatal("snapshot entries share nested source containers")
	}
	data["compound"].(map[string]any)["text"] = "later source change"
	if fresh[0].Data["compound"].(map[string]any)["text"] != "original" {
		t.Fatal("source change mutated old snapshot")
	}
}

// TestChunkClonePreservesBlockEntitiesRoundTrip verifies the recorder's complete
// clone, encode, and network-decode route, including a paired container's NBT.
func TestChunkClonePreservesBlockEntitiesRoundTrip(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	ch := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	pos := cube.Pos{3, 64, 5}
	original := map[string]any{"id": "Chest", "pairx": int32(4), "pairz": int32(5), "Items": []any{map[string]any{"Name": "minecraft:stone", "Count": byte(4)}}}
	ch.SetBlockEntityData(pos, original)
	cloned := ch.Clone()
	original["Items"].([]any)[0].(map[string]any)["Count"] = byte(63)
	ch.SetBlockEntityData(pos, nil)
	entities := cloned.BlockEntities()
	if len(entities) != 1 || entities[0].Data["Items"].([]any)[0].(map[string]any)["Count"] != byte(4) {
		t.Fatalf("clone lost or aliased NBT: %#v", entities)
	}
	data := chunk.Encode(cloned, chunk.NetworkEncoding)
	raw, err := chunk.EncodeLevelChunkPayload(data, entities)
	if err != nil {
		t.Fatal(err)
	}
	_, decoded, err := chunk.NetworkDecodeWithBlockEntities(world.DefaultBlockRegistry, raw, len(data.SubChunks), world.Overworld.Range())
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 || decoded[0].Pos != pos || decoded[0].Data["pairx"] != int32(4) || decoded[0].Data["pairz"] != int32(5) {
		t.Fatalf("lost block-entity identity: %#v", decoded)
	}
	if !reflect.DeepEqual(decoded[0].Data["Items"], entities[0].Data["Items"]) {
		t.Fatalf("lost nested inventory: got %#v want %#v", decoded[0].Data["Items"], entities[0].Data["Items"])
	}
}

// TestBlockEntitiesSnapshotConcurrentReplacement exercises the dedicated entity
// lock without claiming that unrelated Chunk terrain methods are synchronized.
func TestBlockEntitiesSnapshotConcurrentReplacement(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	ch := chunk.New(world.DefaultBlockRegistry, world.Overworld.Range())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			ch.SetBlockEntityData(cube.Pos{1, 64, 1}, map[string]any{"counter": int32(i), "list": []int32{int32(i)}})
		}
	}()
	for i := 0; i < 500; i++ {
		for _, entry := range ch.BlockEntities() {
			if entry.Data["counter"] != entry.Data["list"].([]int32)[0] {
				t.Fatal("snapshot mixed entity replacements")
			}
			entry.Data["list"].([]int32)[0] = -1
		}
	}
	wg.Wait()
}
