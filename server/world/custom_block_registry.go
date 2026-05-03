package world

import (
	"fmt"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/zaataylor/cartesian/cartesian"
)

var traitLookup = map[string][]any{
	"minecraft:facing_direction": {
		"north", "east", "south", "west", "down", "up",
	},
	"minecraft:cardinal_direction": {
		"north", "east", "south", "west",
	},
	"minecraft:vertical_half": {
		"top", "bottom",
	},
	"minecraft:block_face": {
		"north", "east", "south", "west", "down", "up",
	},
	"minecraft:corner_and_cardinal_direction": {
		"none", "inner_left", "inner_right", "outer_left", "outer_right",
	},
}

// NewCustomBlockRegistry returns an independent registry with default block runtime IDs preserved and custom block
// states appended after the default registry.
func NewCustomBlockRegistry(entries []protocol.BlockEntry) (BlockRegistry, error) {
	DefaultBlockRegistry.Finalize()
	registry := DefaultBlockRegistry.Clone()
	if err := registry.AddCustomBlocks(entries); err != nil {
		return nil, err
	}
	return registry, nil
}

// AddCustomBlocks registers each non-vanilla block entry's permutations as block states on reg.
func AddCustomBlocks(reg BlockRegistry, entries []protocol.BlockEntry) error {
	basicRegistry, _ := reg.(*BasicBlockRegistry)
	scratch := make([]byte, 0, 0xff)
	for _, entry := range entries {
		if namespace, _, _ := strings.Cut(entry.Name, ":"); namespace == "minecraft" {
			continue
		}

		propertyNames, propertyValues, err := customBlockPropertySpace(entry)
		if err != nil {
			return err
		}

		permutations := [][]any{nil}
		if len(propertyValues) > 0 {
			permutations = cartesian.NewCartesianProduct(propertyValues).Values()
		}

		for _, values := range permutations {
			properties := make(map[string]any, len(values))
			for i, value := range values {
				properties[propertyNames[i]] = value
			}
			state := BlockState{Name: entry.Name, Properties: properties}
			if basicRegistry != nil {
				var err error
				scratch, err = basicRegistry.addCustomBlockState(state, scratch)
				if err != nil {
					return err
				}
				continue
			}
			reg.RegisterBlockState(state)
		}
	}
	return nil
}

// AddCustomBlocks registers each non-vanilla block entry's permutations as block states on br.
func (br *BasicBlockRegistry) AddCustomBlocks(entries []protocol.BlockEntry) error {
	return AddCustomBlocks(br, entries)
}

func customBlockPropertySpace(entry protocol.BlockEntry) ([]string, []any, error) {
	var propertyNames []string
	var propertyValues []any

	props, ok := entry.Properties["properties"].([]any)
	if ok {
		for i, rawV := range props {
			v, ok := rawV.(map[string]any)
			if !ok {
				return nil, nil, fmt.Errorf("expected property at index %d to be map[string]any, got %T", i, rawV)
			}
			name, ok := v["name"].(string)
			if !ok {
				return nil, nil, fmt.Errorf("expected property name to be string, got %T", v["name"])
			}
			enum, ok := v["enum"]
			if !ok {
				return nil, nil, fmt.Errorf("expected property %s enum to be present", name)
			}
			propertyNames = append(propertyNames, name)
			propertyValues = append(propertyValues, enum)
		}
	}

	traits, ok := entry.Properties["traits"].([]any)
	if ok {
		for i, rawTrait := range traits {
			trait, ok := rawTrait.(map[string]any)
			if !ok {
				return nil, nil, fmt.Errorf("expected trait at index %d to be map[string]any, got %T", i, rawTrait)
			}
			enabledStates, ok := trait["enabled_states"].(map[string]any)
			if !ok {
				return nil, nil, fmt.Errorf("expected enabled_states to be map[string]any, got %T", trait["enabled_states"])
			}
			for k, rawEnabled := range enabledStates {
				if !strings.ContainsRune(k, ':') {
					k = "minecraft:" + k
				}
				if !customBlockTraitEnabled(rawEnabled) {
					continue
				}
				v, ok := traitLookup[k]
				if !ok {
					return nil, nil, fmt.Errorf("unresolved trait %s", k)
				}

				propertyNames = append(propertyNames, k)
				propertyValues = append(propertyValues, v)
			}
		}
	}

	return propertyNames, propertyValues, nil
}

func customBlockTraitEnabled(v any) bool {
	switch v := v.(type) {
	case bool:
		return v
	case uint8:
		return v != 0
	case int32:
		return v != 0
	default:
		return false
	}
}
