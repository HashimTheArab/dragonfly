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

func TestNetworkBlock_Components(t *testing.T) {
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
	if got := b.(Frictional).Friction(); math.Abs(got-0.8) > 1e-7 {
		t.Fatalf("friction=%v", got)
	}
	if got := BreakDuration(b, item.Stack{}, BreakContext{}); got != 2*time.Second {
		t.Fatalf("mining duration=%v", got)
	}
	if got := BreakDuration(b, item.Stack{}, BreakContext{Airborne: true}); got != 10*time.Second {
		t.Fatalf("airborne duration=%v", got)
	}
	if _, ok := b.(world.NBTer); ok {
		t.Fatal("ordinary custom block must not create a block entity")
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
	if BreakDuration(open, item.Stack{}, BreakContext{}) != math.MaxInt64 || BreakDuration(closed, item.Stack{}, BreakContext{}) != 3*time.Second {
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

func TestNetworkBlock_PreservesStatesWithUnsupportedBehaviour(t *testing.T) {
	for _, props := range []map[string]any{
		{"components": map[string]any{"minecraft:friction": map[string]any{"value": float32(math.NaN())}}},
		{"components": map[string]any{"minecraft:collision_box": map[string]any{"origin": []float32{0, 0}, "size": []float32{16, 16, 16}}}},
		{"permutations": []any{map[string]any{"condition": "q.is_sprinting", "components": map[string]any{}}}},
	} {
		r := world.NewBlockRegistry()
		if err := AddCustomBlocks(r, []protocol.BlockEntry{{Name: "test:bad", Properties: props}, {Name: "test:good", Properties: map[string]any{"components": map[string]any{"minecraft:destructible_by_mining": map[string]any{"value": float32(1)}}}}}); err != nil {
			t.Fatal(err)
		}
		r.Finalize()
		b, ok := r.BlockByName("test:bad", nil)
		if !ok {
			t.Fatal("unsupported behaviour lost state identity")
		}
		unresolved, ok := b.(UnresolvedNetworkBlock)
		if !ok || unresolved.BehaviourError() == nil {
			t.Fatal("missing unresolved behaviour diagnostic")
		}
		if _, ok := b.(Breakable); ok {
			t.Fatal("unsupported state is locally mineable")
		}
		good, ok := r.BlockByName("test:good", nil)
		if !ok || BreakDuration(good, item.Stack{}, BreakContext{}) != time.Second {
			t.Fatal("unsupported neighbour affected supported state")
		}
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

func TestNetworkBlock_CollisionFaces(t *testing.T) {
	for _, axis := range cube.Axes() {
		low, high := axis.Faces()
		size := []float32{16, 16, 16}
		for i, component := range axis.Vec3() {
			if component != 0 {
				size[i] = 8
			}
		}
		b := testNetworkBlock(t, map[string]any{"components": map[string]any{"minecraft:collision_box": map[string]any{"origin": []float32{-8, 0, -8}, "size": size}}}, nil)
		for _, face := range cube.Faces() {
			if got, want := b.Model().FaceSolid(cube.Pos{}, face, nil), face == low; got != want {
				t.Fatalf("axis=%v face=%v solid=%v want %v (opposite=%v)", axis, face, got, want, high)
			}
		}
	}
}

func TestNetworkBlock_MultipleTallCollisionBoxes(t *testing.T) {
	b := testNetworkBlock(t, map[string]any{"components": map[string]any{"minecraft:collision_box": map[string]any{"boxes": []map[string]any{
		{"minX": float32(0), "minY": float32(0), "minZ": float32(0), "maxX": float32(4), "maxY": float32(24), "maxZ": float32(16)},
		{"minX": float32(12), "minY": float32(0), "minZ": float32(0), "maxX": float32(16), "maxY": float32(24), "maxZ": float32(16)},
	}}}}, nil)
	boxes := b.Model().BBox(cube.Pos{}, nil)
	if len(boxes) != 2 || boxes[0] != cube.Box(0, 0, 0, 0.25, 1.5, 1) || boxes[1] != cube.Box(0.75, 0, 0, 1, 1.5, 1) {
		t.Fatalf("tall multipart collision=%v", boxes)
	}
	if _, ok := b.(world.NBTer); ok {
		t.Fatal("non-mineable custom block must not create a block entity")
	}
}

// TestNetworkBlock_DocumentedDefaults checks public mining and friction semantics.
func TestNetworkBlock_DocumentedDefaults(t *testing.T) {
	for _, raw := range []any{nil, true, map[string]any{}, map[string]any{"seconds_to_destroy": float32(0)}} {
		components := map[string]any{}
		if raw != nil {
			components["minecraft:destructible_by_mining"] = raw
		}
		b := testNetworkBlock(t, map[string]any{"components": components}, nil)
		if got := BreakDuration(b, item.Stack{}, BreakContext{}); got != 0 {
			t.Fatalf("default %v: %v", raw, got)
		}
		if got := b.(Frictional).Friction(); got != 0.6 {
			t.Fatalf("default friction: %v", got)
		}
	}
	for _, raw := range []any{float32(0.02), map[string]any{"value": float32(0.02)}} {
		b := testNetworkBlock(t, map[string]any{"components": map[string]any{"minecraft:friction": raw, "minecraft:destructible_by_mining": map[string]any{"seconds_to_destroy": float32(3)}}}, nil)
		if got := b.(Frictional).Friction(); math.Abs(got-0.98) > 1e-7 {
			t.Fatalf("ice friction: %v", got)
		}
		if got := BreakDuration(b, item.Stack{}, BreakContext{}); got != 3*time.Second {
			t.Fatalf("documented seconds: %v", got)
		}
	}
}

// TestNetworkBlock_UnverifiedMiningRules preserves definitions without guessing item timing.
func TestNetworkBlock_UnverifiedMiningRules(t *testing.T) {
	for _, descriptor := range []any{"minecraft:iron_pickaxe", map[string]any{"Name": "minecraft:iron_pickaxe"}} {
		b := testNetworkBlock(t, map[string]any{"components": map[string]any{"minecraft:destructible_by_mining": map[string]any{"value": float32(10), "item_specific_speeds": []any{map[string]any{"item": descriptor, "destroy_speed": float32(5)}}}}}, nil)
		if _, ok := b.(UnresolvedNetworkBlock); !ok {
			t.Fatal("unverified item speeds were used for prediction")
		}
		if got := BreakDuration(b, item.Stack{}, BreakContext{}); got != math.MaxInt64 {
			t.Fatalf("unresolved mining duration: %v", got)
		}
	}
}

// TestNetworkBlock_MiningDisabledAndInvalidValues checks explicit disable and numeric boundaries.
func TestNetworkBlock_MiningDisabledAndInvalidValues(t *testing.T) {
	for _, raw := range []any{false, map[string]any{"value": float32(-1)}} {
		b := testNetworkBlock(t, map[string]any{"components": map[string]any{"minecraft:destructible_by_mining": raw}}, nil)
		if got := BreakDuration(b, item.Stack{}, BreakContext{}); got != math.MaxInt64 {
			t.Fatalf("disabled mining: %v", got)
		}
	}
	for _, friction := range []float32{-0.01, 0.91, 1, float32(math.Inf(1))} {
		b := testNetworkBlock(t, map[string]any{"components": map[string]any{"minecraft:friction": friction}}, nil)
		if _, ok := b.(UnresolvedNetworkBlock); !ok {
			t.Fatalf("accepted friction outside public range: %v", friction)
		}
	}
}
