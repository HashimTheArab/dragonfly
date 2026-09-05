package block

import (
	"math"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// testNetworkBlock resolves a real registry entry, including its optional state values.
func testNetworkBlock(t *testing.T, properties map[string]any, state map[string]any) world.Block {
	t.Helper()
	world.DefaultBlockRegistry.Finalize()
	r := world.NewBlockRegistry()
	if err := AddCustomBlocks(r, []protocol.BlockEntry{{Name: "test:component_block", Properties: properties}}); err != nil {
		t.Fatal(err)
	}
	r.Finalize()
	b, ok := r.BlockByName("test:component_block", state)
	if !ok {
		t.Fatal("missing custom state")
	}
	return b
}

func TestNetworkBlock_ComponentsAndNBT(t *testing.T) {
	b := testNetworkBlock(t, map[string]any{"components": map[string]any{
		"minecraft:collision_box":          map[string]any{"enabled": uint8(0)},
		"minecraft:selection_box":          map[string]any{"origin": []float32{-8, 0, -8}, "size": []float32{16, 8, 16}},
		"minecraft:friction":               map[string]any{"value": float32(0.2)},
		"minecraft:destructible_by_mining": map[string]any{"value": float32(2)},
		"minecraft:light_emission":         map[string]any{"emission": int32(7)},
		"minecraft:light_dampening":        map[string]any{"lightLevel": int32(0)},
	}}, nil)
	if boxes := b.Model().BBox(cube.Pos{}, nil); len(boxes) != 0 {
		t.Fatalf("non-solid block has collision: %v", boxes)
	}
	if b.Model().FaceSolid(cube.Pos{}, cube.FaceUp, nil) {
		t.Fatal("non-solid block supports attachments")
	}
	selection := b.Model().(interface {
		SelectionBBox(cube.Pos, world.BlockSource) []cube.BBox
	}).SelectionBBox(cube.Pos{}, nil)
	if len(selection) != 1 || selection[0] != cube.Box(0, 0, 0, 1, 0.5, 1) {
		t.Fatalf("selection=%v", selection)
	}
	if got := b.(Frictional).Friction(); math.Abs(got-0.2) > 1e-7 {
		t.Fatalf("friction=%v", got)
	}
	if got := BreakDuration(b, item.Stack{}, BreakContext{}); got != 3*time.Second {
		t.Fatalf("mining duration=%v", got)
	}
	if got := BreakDuration(b, item.Stack{}, BreakContext{Airborne: true}); got != 15*time.Second {
		t.Fatalf("airborne duration=%v", got)
	}
	attached := b.(world.NBTer).DecodeNBT(map[string]any{"test": int32(7)}).(world.Block)
	if got := BreakDuration(attached, item.Stack{}, BreakContext{}); got != 3*time.Second {
		t.Fatalf("NBT lost mining implementation: %v", got)
	}
	if len(b.(world.NBTer).EncodeNBT()) != 0 {
		t.Fatal("NBT changed registry block")
	}
	if b.(LightEmitter).LightEmissionLevel() != 7 || b.(LightDiffuser).LightDiffusionLevel() != 0 {
		t.Fatal("lighting not installed")
	}
}

func TestNetworkBlock_PermutationsAndIsolation(t *testing.T) {
	props := map[string]any{
		"properties": []any{map[string]any{"name": "test:open", "enum": []uint8{0, 1}}},
		"components": map[string]any{"minecraft:destructible_by_mining": map[string]any{"value": float32(3)}},
		"permutations": []any{map[string]any{
			"condition":  "q.block_state('test:open') == true",
			"components": map[string]any{"minecraft:collision_box": map[string]any{"enabled": uint8(0)}, "minecraft:destructible_by_mining": map[string]any{"value": float32(-1)}},
		}},
	}
	open := testNetworkBlock(t, props, map[string]any{"test:open": uint8(1)})
	closed := testNetworkBlock(t, props, map[string]any{"test:open": uint8(0)})
	if len(open.Model().BBox(cube.Pos{}, nil)) != 0 || len(closed.Model().BBox(cube.Pos{}, nil)) != 1 {
		t.Fatal("permutation shapes not independent")
	}
	if BreakDuration(open, item.Stack{}, BreakContext{}) != math.MaxInt64 || BreakDuration(closed, item.Stack{}, BreakContext{}) != 4500*time.Millisecond {
		t.Fatal("permutation mining not independent")
	}
	_, states := closed.EncodeBlock()
	states["test:open"] = uint8(1)
	_, states = closed.EncodeBlock()
	if states["test:open"] != uint8(0) {
		t.Fatal("caller mutated registry state")
	}
	boxes := closed.Model().BBox(cube.Pos{}, nil)
	boxes[0] = cube.BBox{}
	if closed.Model().BBox(cube.Pos{}, nil)[0] == (cube.BBox{}) {
		t.Fatal("caller mutated registry shape")
	}
}

func TestNetworkBlock_BoxFormsAndTransforms(t *testing.T) {
	for _, test := range []struct {
		name      string
		collision any
		transform any
		want      cube.BBox
	}{
		{name: "absolute", collision: map[string]any{"boxes": []map[string]any{{"minX": float32(0), "minY": float32(0), "minZ": float32(0), "maxX": float32(16), "maxY": float32(8), "maxZ": float32(16)}}}, want: cube.Box(0, 0, 0, 1, 0.5, 1)},
		{name: "rotate", collision: map[string]any{"origin": []float32{-8, 0, -8}, "size": []float32{16, 8, 16}}, transform: map[string]any{"RZ": int32(1)}, want: cube.Box(0.5, 0, 0, 1, 1, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			components := map[string]any{"minecraft:collision_box": test.collision}
			if test.transform != nil {
				components["minecraft:transformation"] = test.transform
			}
			b := testNetworkBlock(t, map[string]any{"components": components}, nil)
			if got := b.Model().BBox(cube.Pos{}, nil); len(got) != 1 || got[0] != test.want {
				t.Fatalf("boxes=%v want %v", got, test.want)
			}
		})
	}
}

func TestNetworkBlock_RejectsUnknownConditionAndMalformedComponents(t *testing.T) {
	for _, props := range []map[string]any{
		{"components": map[string]any{"minecraft:friction": map[string]any{"value": float32(math.NaN())}}},
		{"components": map[string]any{"minecraft:collision_box": map[string]any{"origin": []float32{0, 0}, "size": []float32{16, 16, 16}}}},
		{"permutations": []any{map[string]any{"condition": "q.is_sprinting", "components": map[string]any{}}}},
	} {
		r := world.NewBlockRegistry()
		if err := AddCustomBlocks(r, []protocol.BlockEntry{{Name: "test:bad", Properties: props}}); err == nil {
			t.Fatalf("accepted invalid components: %v", props)
		}
	}
}

func TestNetworkBlock_ItemSpecificHardness(t *testing.T) {
	props := map[string]any{"components": map[string]any{"minecraft:destructible_by_mining": map[string]any{
		"value": float32(4), "item_specific_speeds": []any{
			map[string]any{"item": map[string]any{"Name": "minecraft:diamond_pickaxe", "Aux": int16(0)}, "destroy_speed": float32(1)},
			map[string]any{"item": map[string]any{"Tags": "q.any_tag('minecraft:is_pickaxe', 'minecraft:is_axe')"}, "destroy_speed": float32(2)},
		}}}}
	b := testNetworkBlock(t, props, nil)
	for _, test := range []struct {
		stack item.Stack
		want  time.Duration
	}{
		{item.Stack{}, 6 * time.Second},
		{item.NewStack(item.Pickaxe{Tier: item.ToolTierDiamond}, 1), 1500 * time.Millisecond},
		{item.NewStack(item.Pickaxe{Tier: item.ToolTierIron}, 1), 3 * time.Second},
		{item.NewStack(item.Shovel{Tier: item.ToolTierIron}, 1), 6 * time.Second},
	} {
		if got := BreakDuration(b, test.stack, BreakContext{}); got != test.want {
			t.Fatalf("item %T duration=%v want %v", test.stack.Item(), got, test.want)
		}
	}
	props["blockTags"] = []string{"minecraft:is_pickaxe_item_destructible"}
	b = testNetworkBlock(t, props, nil)
	if got := BreakDuration(b, item.NewStack(item.Pickaxe{Tier: item.ToolTierDiamond}, 1), BreakContext{}); got != 200*time.Millisecond {
		t.Fatalf("tagged diamond pickaxe duration=%v", got)
	}
}

func TestNetworkBlock_LegacyBlockPropertyQuery(t *testing.T) {
	for _, query := range []string{"q.block_property", "query.block_property", "q.block_state", "query.block_state"} {
		got, err := evaluateBlockCondition(query+"('server:variant') == 1", map[string]any{"server:variant": int32(1)})
		if err != nil || !got {
			t.Fatalf("%s: got %v, error %v", query, got, err)
		}
	}
}
