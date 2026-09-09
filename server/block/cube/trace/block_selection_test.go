package trace_test

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestBlockSelectionIntercept_UsesSelectionWithoutCollision(t *testing.T) {
	registry, err := block.NewCustomBlockRegistry([]protocol.BlockEntry{{Name: "test:selectable", Properties: map[string]any{
		"components": map[string]any{"minecraft:collision_box": map[string]any{"enabled": uint8(0)}, "minecraft:selection_box": map[string]any{"origin": []float32{-8, 0, -8}, "size": []float32{16, 8, 16}}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	b, ok := registry.BlockByName("test:selectable", nil)
	if !ok {
		t.Fatal("missing state")
	}
	start, end := mgl64.Vec3{0.5, 0.25, -1}, mgl64.Vec3{0.5, 0.25, 2}
	if _, ok := trace.BlockIntercept(cube.Pos{}, nil, b, start, end); ok {
		t.Fatal("physical trace hit a non-solid block")
	}
	hit, ok := trace.BlockSelectionIntercept(cube.Pos{}, nil, b, start, end)
	if !ok || hit.Face() != cube.FaceNorth {
		t.Fatalf("selection hit=%v found=%t", hit, ok)
	}
}
