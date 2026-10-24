package customblock

import (
	"reflect"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/nbt"
)

func TestCustomBlock_CloneEncodedPropertiesWithMaterial(t *testing.T) {
	t.Parallel()

	sourceMaterial := map[string]any{"materials": map[string]any{"*": map[string]any{"texture": "source"}}}
	properties := map[string]any{
		"unknown_top_level":  map[string]any{"preserved": "yes"},
		"vanilla_block_data": map[string]any{"block_id": int32(12), "unknown_sibling": int32(7)},
		"properties": []any{map[string]any{
			"name": "server:variant",
			"enum": []int32{0, 1},
		}},
		"traits": []any{map[string]any{
			"name":              "minecraft:placement_direction",
			"enabled_states":    []any{"minecraft:cardinal_direction"},
			"y_rotation_offset": int32(0),
		}},
		"components": map[string]any{
			"minecraft:geometry":           map[string]any{"identifier": "geometry.server.shaped"},
			"minecraft:collision_box":      map[string]any{"enabled": uint8(1)},
			"minecraft:material_instances": sourceMaterial,
			"server:unknown_component":     map[string]any{"preserved": uint8(1)},
		},
		"permutations": []any{
			map[string]any{
				"condition": "q.block_state('server:variant') == 0",
				"components": map[string]any{
					"minecraft:transformation":     map[string]any{"RY": int32(1)},
					"minecraft:material_instances": sourceMaterial,
					"server:unknown_component":     map[string]any{"preserved": uint8(1)},
				},
			},
			map[string]any{
				"condition":  "q.block_state('server:variant') == 1",
				"components": map[string]any{"minecraft:transformation": map[string]any{"RY": int32(2)}},
			},
		},
	}
	data, err := nbt.Marshal(properties)
	if err != nil {
		t.Fatalf("marshal source properties: %v", err)
	}
	var original map[string]any
	if err := nbt.Unmarshal(data, &original); err != nil {
		t.Fatalf("unmarshal source properties: %v", err)
	}
	material := NewMaterial("replacement", BlendRenderMethod()).WithoutAmbientOcclusion()

	cloned, err := CloneEncodedPropertiesWithMaterial(properties, 34, material)
	if err != nil {
		t.Fatalf("CloneEncodedPropertiesWithMaterial() error = %v", err)
	}
	if got := cloned["vanilla_block_data"].(map[string]any)["block_id"]; got != int32(34) {
		t.Fatalf("cloned block ID = %v, want 34", got)
	}
	if got := cloned["vanilla_block_data"].(map[string]any)["unknown_sibling"]; got != int32(7) {
		t.Fatalf("cloned vanilla block sibling = %v, want 7", got)
	}
	if !reflect.DeepEqual(cloned["unknown_top_level"], properties["unknown_top_level"]) {
		t.Fatal("clone did not preserve unknown top-level data")
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
	if !reflect.DeepEqual(properties, original) {
		t.Fatal("source properties mutated")
	}
}

func TestCustomBlock_CloneEncodedPropertiesWithMaterialRejectsMissingGeometry(t *testing.T) {
	t.Parallel()

	properties := map[string]any{"components": map[string]any{"minecraft:collision_box": map[string]any{}}}
	cloned, err := CloneEncodedPropertiesWithMaterial(properties, 34, NewMaterial("replacement", BlendRenderMethod()))
	if err == nil || cloned != nil {
		t.Fatalf("CloneEncodedPropertiesWithMaterial() = (%v, %v), want (nil, error)", cloned, err)
	}
}

func TestCustomBlock_CloneEncodedPropertiesWithMaterialAddsMissingVanillaData(t *testing.T) {
	t.Parallel()

	properties := map[string]any{
		"components": map[string]any{
			"minecraft:geometry": map[string]any{"identifier": "geometry.server.shaped"},
		},
	}
	cloned, err := CloneEncodedPropertiesWithMaterial(
		properties, 34, NewMaterial("replacement", BlendRenderMethod()),
	)
	if err != nil {
		t.Fatalf("CloneEncodedPropertiesWithMaterial() error = %v", err)
	}
	if got := cloned["vanilla_block_data"].(map[string]any)["block_id"]; got != int32(34) {
		t.Fatalf("cloned block ID = %v, want 34", got)
	}
	if _, ok := properties["vanilla_block_data"]; ok {
		t.Fatal("source properties gained vanilla block data")
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
