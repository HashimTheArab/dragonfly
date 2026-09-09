package block

import (
	"fmt"
	"sort"
	"strings"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
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

const maxCustomBlockStates = 1 << 16

type customBlockStateSpace struct {
	entry      protocol.BlockEntry
	properties []string
	values     [][]any
}

// NewCustomBlockRegistry returns an independent registry with default block runtime IDs preserved and custom block
// states appended after the default registry.
func NewCustomBlockRegistry(entries []protocol.BlockEntry) (world.BlockRegistry, error) {
	world.DefaultBlockRegistry.Finalize()
	registry := world.DefaultBlockRegistry.Clone()
	if err := AddCustomBlocks(registry, entries); err != nil {
		return nil, err
	}
	return registry, nil
}

// AddCustomBlocks appends StartGame network block states while preserving existing runtime IDs.
// Entries contain compiled network components, not behaviour-pack JSON: for example, mining
// uses hardness in value rather than seconds_to_destroy. Unsupported behaviour produces an
// UnresolvedNetworkBlock without failing state registration. Invalid state declarations still
// return an error; in that case, discard the registry.
func AddCustomBlocks(registry world.BlockRegistry, entries []protocol.BlockEntry) error {
	basicRegistry, ok := registry.(*world.BasicBlockRegistry)
	if !ok {
		return fmt.Errorf("unsupported block registry type %T", registry)
	}
	spaces := make([]customBlockStateSpace, 0, len(entries))
	total := 0
	for _, entry := range entries {
		if namespace, _, _ := strings.Cut(entry.Name, ":"); namespace == "minecraft" {
			continue
		}

		propertyNames, propertyValues, err := customBlockPropertySpace(entry)
		if err != nil {
			return fmt.Errorf("custom block %s: %w", entry.Name, err)
		}
		count := 1
		for _, values := range propertyValues {
			if count > (maxCustomBlockStates-total)/len(values) {
				return fmt.Errorf("custom block %s: states exceed limit of %d", entry.Name, maxCustomBlockStates)
			}
			count *= len(values)
		}
		if count > maxCustomBlockStates-total {
			return fmt.Errorf("custom block %s: states exceed limit of %d", entry.Name, maxCustomBlockStates)
		}
		total += count
		spaces = append(spaces, customBlockStateSpace{entry, propertyNames, propertyValues})
	}

	for _, space := range spaces {
		err := forEachCustomBlockState(space.properties, space.values, func(properties map[string]any) error {
			b, err := decodeNetworkBlock(space.entry, properties)
			if err != nil {
				b = unresolvedBlock(space.entry, properties, err)
			}
			return basicRegistry.AppendNetworkBlock(b)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// customBlockPropertySpace reads the state declarations and traits in network order.
func customBlockPropertySpace(entry protocol.BlockEntry) ([]string, [][]any, error) {
	var propertyNames []string
	var propertyValues [][]any

	props, err := customBlockMaps(entry.Properties["properties"], "properties")
	if err != nil {
		return nil, nil, err
	}
	for _, v := range props {
		name, ok := v["name"].(string)
		if !ok {
			return nil, nil, fmt.Errorf("expected property name to be string, got %T", v["name"])
		}
		enum, ok := customBlockEnumValues(v["enum"])
		if !ok {
			return nil, nil, fmt.Errorf("expected property %s enum to be a list, got %T", name, v["enum"])
		}
		if len(enum) == 0 {
			return nil, nil, fmt.Errorf("expected property %s enum to contain at least one value", name)
		}
		propertyNames = append(propertyNames, name)
		propertyValues = append(propertyValues, enum)
	}

	traits, err := customBlockMaps(entry.Properties["traits"], "traits")
	if err != nil {
		return nil, nil, err
	}
	for _, trait := range traits {
		enabledStates, ok := trait["enabled_states"].(map[string]any)
		if !ok {
			// minecraft:connection is the one trait whose enabled_states arrives as a
			// TAG_Int bitmask instead of a map; it declares the four directional
			// booleans. Byte values, so chunk-palette lookups match.
			if _, isMask := trait["enabled_states"].(int32); isMask {
				name, nameOK := trait["name"].(string)
				if !nameOK {
					return nil, nil, fmt.Errorf("expected trait name to be string, got %T", trait["name"])
				}
				if name != "minecraft:connection" {
					return nil, nil, fmt.Errorf("unresolved bitmask trait %s", name)
				}
				for _, state := range []string{
					"minecraft:connection_north",
					"minecraft:connection_south",
					"minecraft:connection_west",
					"minecraft:connection_east",
				} {
					propertyNames = append(propertyNames, state)
					propertyValues = append(propertyValues, []any{uint8(0), uint8(1)})
				}
				continue
			}
			return nil, nil, fmt.Errorf("expected enabled_states to be map[string]any or int32, got %T", trait["enabled_states"])
		}
		keys := make([]string, 0, len(enabledStates))
		for k := range enabledStates {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			rawEnabled := enabledStates[k]
			if !strings.ContainsRune(k, ':') {
				k = "minecraft:" + k
			}
			enabled, err := customBlockTraitEnabled(rawEnabled)
			if err != nil {
				return nil, nil, fmt.Errorf("trait %s: %w", k, err)
			}
			if !enabled {
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

	return propertyNames, propertyValues, nil
}

// customBlockEnumValues coerces a property enum to []any. NBT decoders produce typed
// slices for homogeneous lists (Geyser encodes boolean enums as []uint8), so scalar
// slice types are widened element-wise; the element values keep their original type.
func customBlockEnumValues(value any) ([]any, bool) {
	switch value := value.(type) {
	case []any:
		return value, true
	case []uint8:
		return anySlice(value), true
	case []int32:
		return anySlice(value), true
	case []int64:
		return anySlice(value), true
	case []string:
		return anySlice(value), true
	case []float32:
		return anySlice(value), true
	case []float64:
		return anySlice(value), true
	default:
		return nil, false
	}
}

// anySlice widens a typed slice to []any.
func anySlice[T any](values []T) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

// customBlockMaps reads an NBT compound list without accepting malformed entries.
func customBlockMaps(value any, field string) ([]map[string]any, error) {
	switch value := value.(type) {
	case nil:
		return nil, nil
	case []map[string]any:
		return value, nil
	case []any:
		values := make([]map[string]any, len(value))
		for i, entry := range value {
			mapped, ok := entry.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("expected %s entry at index %d to be map[string]any, got %T", field, i, entry)
			}
			values[i] = mapped
		}
		return values, nil
	default:
		return nil, fmt.Errorf("expected %s to be a list of maps, got %T", field, value)
	}
}

// forEachCustomBlockState enumerates the Cartesian product in palette order.
func forEachCustomBlockState(names []string, valueSets [][]any, yield func(map[string]any) error) error {
	properties := make(map[string]any, len(names))
	var visit func(int) error
	visit = func(index int) error {
		if index == len(names) {
			state := make(map[string]any, len(properties))
			for name, value := range properties {
				state[name] = value
			}
			return yield(state)
		}
		for _, value := range valueSets[index] {
			properties[names[index]] = value
			if err := visit(index + 1); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(0)
}

// customBlockTraitEnabled decodes the supported NBT boolean representations.
func customBlockTraitEnabled(v any) (bool, error) {
	switch v := v.(type) {
	case bool:
		return v, nil
	case uint8:
		return v != 0, nil
	case int32:
		return v != 0, nil
	default:
		return false, fmt.Errorf("expected enabled flag to be bool, uint8, or int32, got %T", v)
	}
}
