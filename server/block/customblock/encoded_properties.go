package customblock

import "github.com/sandertv/gophertunnel/minecraft/nbt"

// CloneEncodedPropertiesWithMaterial deep-clones encoded custom-block properties and
// replaces their rendered material while preserving geometry, collision, selection,
// transformations, properties and permutations. The returned bool reports whether the
// properties describe a block with geometry and can therefore use the material.
//
// blockID replaces the encoded vanilla block ID in the clone. Permutations that supply
// their own material instances are replaced too, so they cannot override material.
func CloneEncodedPropertiesWithMaterial(
	properties map[string]any, blockID int32, material Material,
) (map[string]any, bool, error) {
	if !EncodedPropertiesHaveGeometry(properties) {
		return nil, false, nil
	}
	data, err := nbt.Marshal(properties)
	if err != nil {
		return nil, false, err
	}
	var cloned map[string]any
	if err := nbt.Unmarshal(data, &cloned); err != nil {
		return nil, false, err
	}

	components := cloned["components"].(map[string]any)
	cloned["vanilla_block_data"] = map[string]any{"block_id": blockID}
	components["minecraft:material_instances"] = encodedMaterialInstances(material)
	permutations, _ := cloned["permutations"].([]any)
	for _, raw := range permutations {
		permutation, _ := raw.(map[string]any)
		permutationComponents, _ := permutation["components"].(map[string]any)
		if _, overridesMaterial := permutationComponents["minecraft:material_instances"]; overridesMaterial {
			permutationComponents["minecraft:material_instances"] = encodedMaterialInstances(material)
		}
	}
	return cloned, true, nil
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

func encodedMaterialInstances(material Material) map[string]any {
	return map[string]any{
		"mappings":  map[string]any{},
		"materials": map[string]any{"*": material.Encode()},
	}
}
