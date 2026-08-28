package customblock

import (
	"reflect"
	"testing"
)

func TestCustomBlock_CloneEncodedPropertiesWithMaterial(t *testing.T) {
	t.Parallel()

	sourceMaterial := map[string]any{"materials": map[string]any{"*": map[string]any{"texture": "source"}}}
	properties := map[string]any{
		"vanilla_block_data": map[string]any{"block_id": int32(12)},
		"properties": []any{map[string]any{
			"name": "server:variant",
			"enum": []int32{0, 1},
		}},
		"components": map[string]any{
			"minecraft:geometry":           map[string]any{"identifier": "geometry.server.shaped"},
			"minecraft:collision_box":      map[string]any{"enabled": uint8(1)},
			"minecraft:material_instances": sourceMaterial,
		},
		"permutations": []any{
			map[string]any{
				"condition": "q.block_state('server:variant') == 0",
				"components": map[string]any{
					"minecraft:transformation":     map[string]any{"RY": int32(1)},
					"minecraft:material_instances": sourceMaterial,
				},
			},
			map[string]any{
				"condition":  "q.block_state('server:variant') == 1",
				"components": map[string]any{"minecraft:transformation": map[string]any{"RY": int32(2)}},
			},
		},
	}
	material := NewMaterial("replacement", BlendRenderMethod()).WithoutAmbientOcclusion()

	cloned, ok, err := CloneEncodedPropertiesWithMaterial(properties, 34, material)
	if err != nil {
		t.Fatalf("CloneEncodedPropertiesWithMaterial() error = %v", err)
	}
	if !ok {
		t.Fatal("CloneEncodedPropertiesWithMaterial() rejected properties with geometry")
	}
	if got := cloned["vanilla_block_data"].(map[string]any)["block_id"]; got != int32(34) {
		t.Fatalf("cloned block ID = %v, want 34", got)
	}
	components := cloned["components"].(map[string]any)
	if got := components["minecraft:geometry"].(map[string]any)["identifier"]; got != "geometry.server.shaped" {
		t.Fatalf("cloned geometry = %v, want source geometry", got)
	}
	assertReplacementMaterial(t, components["minecraft:material_instances"])

	permutations := cloned["permutations"].([]any)
	firstComponents := permutations[0].(map[string]any)["components"].(map[string]any)
	assertReplacementMaterial(t, firstComponents["minecraft:material_instances"])
	secondComponents := permutations[1].(map[string]any)["components"].(map[string]any)
	if _, ok := secondComponents["minecraft:material_instances"]; ok {
		t.Fatal("clone added a material override to a permutation that did not have one")
	}
	if got := properties["vanilla_block_data"].(map[string]any)["block_id"]; got != int32(12) {
		t.Fatalf("source block ID mutated to %v", got)
	}
	if !reflect.DeepEqual(properties["components"].(map[string]any)["minecraft:material_instances"], sourceMaterial) {
		t.Fatal("source material mutated")
	}
}

func TestCustomBlock_CloneEncodedPropertiesWithMaterialRejectsMissingGeometry(t *testing.T) {
	t.Parallel()

	properties := map[string]any{"components": map[string]any{"minecraft:collision_box": map[string]any{}}}
	cloned, ok, err := CloneEncodedPropertiesWithMaterial(properties, 34, NewMaterial("replacement", BlendRenderMethod()))
	if err != nil {
		t.Fatalf("CloneEncodedPropertiesWithMaterial() error = %v", err)
	}
	if ok || cloned != nil {
		t.Fatalf("CloneEncodedPropertiesWithMaterial() = (%v, %t), want (nil, false)", cloned, ok)
	}
}

func TestCustomBlock_EncodedPropertiesHaveGeometry(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		properties map[string]any
		want       bool
	}{
		{name: "missing components"},
		{name: "missing geometry", properties: map[string]any{"components": map[string]any{}}},
		{name: "geometry", properties: map[string]any{
			"components": map[string]any{"minecraft:geometry": map[string]any{}},
		}, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := EncodedPropertiesHaveGeometry(test.properties); got != test.want {
				t.Fatalf("EncodedPropertiesHaveGeometry() = %t, want %t", got, test.want)
			}
		})
	}
}

func assertReplacementMaterial(t *testing.T, raw any) {
	t.Helper()
	component := raw.(map[string]any)
	encoded := component["materials"].(map[string]any)["*"].(map[string]any)
	if got := encoded["texture"]; got != "replacement" {
		t.Fatalf("material texture = %v, want replacement", got)
	}
	if got := encoded["render_method"]; got != "blend" {
		t.Fatalf("material render method = %v, want blend", got)
	}
	if got := encoded["ambient_occlusion"]; got != float32(0) {
		t.Fatalf("material ambient occlusion = %v, want 0", got)
	}
}
