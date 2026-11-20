package world

import (
	"fmt"
	"math/bits"
	"sort"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// traitLookup holds the values of each trait state in the order the client enumerates them: the
// order of its Direction and Facing enums, which vanilla's own block states share.
var traitLookup = map[string][]any{
	"minecraft:facing_direction": {
		"down", "up", "north", "south", "west", "east",
	},
	"minecraft:cardinal_direction": {
		"south", "west", "north", "east",
	},
	"minecraft:vertical_half": {
		"bottom", "top",
	},
	"minecraft:block_face": {
		"down", "up", "north", "south", "west", "east",
	},
	"minecraft:corner_and_cardinal_direction": {
		"none", "inner_left", "inner_right", "outer_left", "outer_right",
	},
}

// maxCustomBlockCost bounds the memory of one registry's custom states, about 100 MB, against a
// hostile palette. A state costs about 3 units and each of its properties 1, at about 70 B a unit.
// The client has no such cap; it only warns past 65536 permutations.
const maxCustomBlockCost = 1_500_000

// customBlockStateSpace is a custom block's states laid out in permutation bits as the client lays
// them out: each state takes the bits of its largest value index, the first added the lowest. The
// client keeps only the low 16 bits, so a state reaching past them never leaves its first value.
type customBlockStateSpace struct {
	name   string
	states []customBlockState
	// spare is the bits below 16 that no state keeps; each value of them repeats every state.
	spare uint32
	known bool // the registry already held the block, so only its missing states are added
}

type customBlockState struct {
	name   string
	values []any
	kept   bool // within the low 16 bits
}

func newCustomBlockStateSpace(name string, names []string, values [][]any) (customBlockStateSpace, error) {
	space := customBlockStateSpace{name: name}
	start := uint32(0)
	for i, stateName := range names {
		n := uint32(max(1, bits.Len(uint(len(values[i])-1))))
		// The client's mask, in its 32-bit arithmetic: zero for a state past bit 16, until the
		// shift wraps for a block of more than 32 bits.
		mask := (0xffff << ((16 - n) & 31) & 0xffff) >> ((16 - (start + n)) & 31)
		kept := start+n <= 16
		if !kept && mask != 0 {
			return space, fmt.Errorf("states need %d bits or more, which the client cannot lay out", start+n)
		}
		if start < 16 && !kept {
			space.spare = 16 - start
		}
		space.states = append(space.states, customBlockState{stateName, values[i], kept})
		start += n
	}
	return space, nil
}

// cost returns the memory units the block's permutations take in a registry.
func (s customBlockStateSpace) cost() int {
	return s.count() * (3 + len(s.states))
}

// count returns how many permutations the client gives the block.
func (s customBlockStateSpace) count() int {
	count := 1 << s.spare
	for _, state := range s.states {
		if state.kept {
			count *= len(state.values)
		}
	}
	return count
}

// forEach yields the block's permutations in the client's order, by permutation index.
func (s customBlockStateSpace) forEach(yield func(map[string]any) error) error {
	properties := make(map[string]any, len(s.states))
	var kept []customBlockState
	for _, state := range s.states {
		if state.kept {
			kept = append(kept, state)
		} else {
			properties[state.name] = state.values[0]
		}
	}
	var visit func(int) error
	visit = func(index int) error {
		if index < 0 {
			state := make(map[string]any, len(properties))
			for name, value := range properties {
				state[name] = value
			}
			return yield(state)
		}
		for _, value := range kept[index].values {
			properties[kept[index].name] = value
			if err := visit(index - 1); err != nil {
				return err
			}
		}
		return nil
	}
	for range 1 << s.spare {
		if err := visit(len(kept) - 1); err != nil {
			return err
		}
	}
	return nil
}

// NewCustomBlockRegistry returns an independent registry with default block runtime IDs preserved and custom block
// states appended after the default registry.
func NewCustomBlockRegistry(entries []protocol.BlockEntry) (BlockRegistry, error) {
	DefaultBlockRegistry.Finalize()
	registry := DefaultBlockRegistry.Clone()
	if err := AddCustomBlocks(registry, entries); err != nil {
		return nil, err
	}
	return registry, nil
}

// AddCustomBlocks appends custom block states to registry while preserving its existing runtime IDs.
func AddCustomBlocks(registry BlockRegistry, entries []protocol.BlockEntry) error {
	basicRegistry, ok := registry.(*BasicBlockRegistry)
	if !ok {
		return fmt.Errorf("unsupported block registry type %T", registry)
	}
	spaces := make([]customBlockStateSpace, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	states, cost := 0, 0
	for _, entry := range entries {
		if namespace, _, _ := strings.Cut(entry.Name, ":"); namespace == "minecraft" {
			continue
		}
		if _, ok := seen[entry.Name]; ok {
			continue
		}
		seen[entry.Name] = struct{}{}

		propertyNames, propertyValues, err := customBlockPropertySpace(entry)
		if err != nil {
			return fmt.Errorf("custom block %s: %w", entry.Name, err)
		}
		space, err := newCustomBlockStateSpace(entry.Name, propertyNames, propertyValues)
		if err != nil {
			return fmt.Errorf("custom block %s: %w", entry.Name, err)
		}
		states += space.count()
		cost += space.cost()
		if cost > maxCustomBlockCost {
			return fmt.Errorf("custom block %s: %d custom states cost %d units, over the limit of %d", entry.Name, states, cost, maxCustomBlockCost)
		}
		space.known = basicRegistry.hasBlock(entry.Name)
		spaces = append(spaces, space)
	}

	scratch := make([]byte, 0, 0xff)
	for _, space := range spaces {
		err := space.forEach(func(properties map[string]any) error {
			var stateErr error
			scratch, stateErr = basicRegistry.addCustomBlockState(BlockState{Name: space.name, Properties: properties}, space.known, scratch)
			return stateErr
		})
		if err != nil {
			return err
		}
	}
	return nil
}

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

	// The client adds trait states to the block before its properties.
	var traitNames []string
	var traitValues [][]any
	traits, err := customBlockMaps(entry.Properties["traits"], "traits")
	if err != nil {
		return nil, nil, err
	}
	for _, trait := range traits {
		if trait["name"] == "minecraft:multi_block" {
			values, err := multiBlockPartValues(trait)
			if err != nil {
				return nil, nil, fmt.Errorf("trait minecraft:multi_block: %w", err)
			}
			if values != nil {
				traitNames = append(traitNames, "minecraft:multi_block_part")
				traitValues = append(traitValues, values)
			}
			continue
		}
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
					traitNames = append(traitNames, state)
					traitValues = append(traitValues, []any{uint8(0), uint8(1)})
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

			traitNames = append(traitNames, k)
			traitValues = append(traitValues, v)
		}
	}

	return append(traitNames, propertyNames...), append(traitValues, propertyValues...), nil
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

// multiBlockPartValues returns the values of minecraft:multi_block_part, one per part, or nil if
// the trait does not enable it. The client stores the part as an int and writes a part of 4 or
// more as 0.
func multiBlockPartValues(trait map[string]any) ([]any, error) {
	enabledStates, _ := trait["enabled_states"].(map[string]any)
	// The client reads the flag only as a byte, and any other tag as off.
	if flag, _ := enabledStates["multi_block"].(uint8); flag == 0 {
		return nil, nil
	}
	parts, ok := trait["part_count"].(uint8)
	if !ok || parts == 0 {
		return nil, fmt.Errorf("expected part_count to be a byte above 0, got %v", trait["part_count"])
	}
	values := make([]any, parts)
	for i := range values {
		values[i] = int32(0)
		if i < 4 {
			values[i] = int32(i)
		}
	}
	return values, nil
}

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
