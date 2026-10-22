package world

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"testing"
)

// TestBlockState_Facing covers each state encoding and rejects invalid values.
func TestBlockState_Facing(t *testing.T) {
	for _, face := range cube.Faces() {
		for _, property := range []string{"minecraft:cardinal_direction", "minecraft:facing_direction", "facing_direction"} {
			var value any = face.String()
			if property == "facing_direction" {
				value = int32(face)
			}
			got, ok := (BlockState{Properties: map[string]any{property: value}}).Facing()
			if !ok || got != face {
				t.Errorf("%s=%v: got %v %v, want %v", property, value, got, ok, face)
			}
		}
	}
	for index, face := range []cube.Face{cube.FaceSouth, cube.FaceWest, cube.FaceNorth, cube.FaceEast} {
		got, ok := (BlockState{Properties: map[string]any{"direction": int32(index)}}).Facing()
		if !ok || got != face {
			t.Errorf("direction=%d: got %v %v, want %v", index, got, ok, face)
		}
	}
	for _, properties := range []map[string]any{
		nil,
		{"minecraft:cardinal_direction": "invalid", "facing_direction": int32(cube.FaceWest)},
		{"minecraft:facing_direction": "invalid", "direction": int32(1)},
		{"facing_direction": int32(-1)},
		{"facing_direction": int32(6)},
		{"facing_direction": "north"},
		{"direction": int32(-1)},
		{"direction": int32(4)},
		{"direction": int(2)},
	} {
		if _, ok := (BlockState{Properties: properties}).Facing(); ok {
			t.Errorf("accepted invalid facing: %v", properties)
		}
	}
	got, ok := (BlockState{Properties: map[string]any{
		"minecraft:cardinal_direction": "east", "minecraft:facing_direction": "west", "facing_direction": int32(cube.FaceNorth), "direction": int32(0),
	}}).Facing()
	if !ok || got != cube.FaceEast {
		t.Fatalf("property precedence: %v %v", got, ok)
	}
}

// TestBlockState_PillarAxis keeps missing and invalid axes distinct from Y.
func TestBlockState_PillarAxis(t *testing.T) {
	for _, axis := range cube.Axes() {
		got, ok := (BlockState{Properties: map[string]any{"pillar_axis": axis.String()}}).PillarAxis()
		if !ok || got != axis {
			t.Errorf("axis=%s: got %v %v", axis, got, ok)
		}
	}
	for _, value := range []any{nil, "invalid", int32(0)} {
		if _, ok := (BlockState{Properties: map[string]any{"pillar_axis": value}}).PillarAxis(); ok {
			t.Errorf("accepted invalid axis: %v", value)
		}
	}
}
