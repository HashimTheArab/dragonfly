package customblock

import (
	"fmt"

	"github.com/sandertv/gophertunnel/minecraft/nbt"
)

// CloneEncodedPropertiesWithMaterial deep-clones encoded custom-block properties and
// replaces their rendered material while preserving geometry, collision, selection,
// transformations, properties and permutations. Properties without geometry are rejected
// because changing their material cannot produce a usable shadow.
//
// blockID replaces the encoded vanilla block ID in the clone. Permutations that supply
// their own material instances are replaced too, so they cannot override material.
func CloneEncodedPropertiesWithMaterial(
	properties map[string]any, blockID int32, material Material,
) (map[string]any, error) {
	if !EncodedPropertiesHaveGeometry(properties) {
		return nil, fmt.Errorf("encoded custom block properties have no geometry")
	}
	data, err := nbt.Marshal(properties)
	if err != nil {
		return nil, err
	}
	var cloned map[string]any
	if err := nbt.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}

	components := cloned["components"].(map[string]any)
	vanillaData, ok := cloned["vanilla_block_data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("encoded custom block properties have no vanilla block data")
	}
	vanillaData["block_id"] = blockID
	replacement := Properties{Textures: map[string]Material{"*": material}}
	components["minecraft:material_instances"] = replacement.MaterialInstances()
	permutations, _ := cloned["permutations"].([]any)
	for _, raw := range permutations {
		permutation, _ := raw.(map[string]any)
		permutationComponents, _ := permutation["components"].(map[string]any)
		if _, overridesMaterial := permutationComponents["minecraft:material_instances"]; overridesMaterial {
			permutationComponents["minecraft:material_instances"] = replacement.MaterialInstances()
		}
	}
	return cloned, nil
}

// EncodedPropertiesHaveGeometry reports whether encoded custom-block properties contain
// a geometry component. Blocks without geometry cannot be visually replaced by changing
// their material instances.
func EncodedPropertiesHaveGeometry(properties map[string]any) bool {
	components, ok := properties["components"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = components["minecraft:geometry"]
	return ok
}
