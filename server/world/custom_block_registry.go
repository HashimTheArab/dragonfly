package world

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/df-mc/dragonfly/server/block/customblock"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// maxNetworkBlockStates bounds the states a palette may add. The client itself only logs past
// 65536 permutations; this limit keeps a hostile palette from exhausting memory, at the cost of
// rejecting a palette the client would load past it.
const maxNetworkBlockStates = 1 << 16

// maxBlockStateBits is the width of a block's packed state data value on the client. States whose
// bits lie past it can only take the values their bits below it encode.
const maxBlockStateBits = 16

// networkBlockDefinition is one StartGame block definition as the client digests it: its name and
// its states in the order the client adds them, trait states before properties.
type networkBlockDefinition struct {
	name   string
	states []customblock.TraitState
	// traits holds the names of the states the block's traits added.
	traits map[string]struct{}
	// vanilla reports whether the entry named the block in the minecraft namespace, which lets it
	// add states to a block an earlier definition registered.
	vanilla bool
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

// AddCustomBlocks appends the states of a StartGame block palette to registry while preserving its
// existing runtime IDs. Entries are read as the client reads them: an entry without
// vanilla_block_data is ignored, a later entry replaces an earlier one with the same name, and
// malformed properties and traits are skipped one element at a time. A block registry already
// holds keeps its own states. It returns an error only for a palette the client cannot load.
func AddCustomBlocks(registry BlockRegistry, entries []protocol.BlockEntry) error {
	basicRegistry, ok := registry.(*BasicBlockRegistry)
	if !ok {
		return fmt.Errorf("unsupported block registry type %T", registry)
	}
	definitions, err := digestNetworkBlocks(entries)
	if err != nil {
		return err
	}
	return addNetworkBlocks(basicRegistry, definitions)
}

// addNetworkBlocks appends the states of the blocks the definitions register, keyed by lowercased
// name as the client keys block types. A later definition of a block an earlier one registered is
// ignored, unless it names the block in the minecraft namespace: the client then adds its trait
// states the block lacks and its properties after the block's states. A block the registry already
// holds keeps its own states. The state limit is checked before the registry changes.
func addNetworkBlocks(registry *BasicBlockRegistry, definitions []networkBlockDefinition) error {
	held := make(map[string]struct{}, len(registry.blockProperties))
	for name := range registry.blockProperties {
		held[asciiLower(name)] = struct{}{}
	}
	blocks := make([]networkBlockDefinition, 0, len(definitions))
	index := make(map[string]int, len(definitions))
	for _, definition := range definitions {
		key := asciiLower(definition.name)
		if _, ok := held[key]; ok {
			continue
		}
		at, registered := index[key]
		if !registered {
			index[key] = len(blocks)
			definition.traits = maps.Clone(definition.traits)
			blocks = append(blocks, definition)
			continue
		}
		if !definition.vanilla {
			continue
		}
		block := &blocks[at]
		block.states = slices.Clone(block.states)
		for _, state := range definition.states {
			if _, trait := definition.traits[state.Name]; trait {
				if _, has := block.traits[state.Name]; has {
					continue
				}
				block.traits[state.Name] = struct{}{}
			}
			block.states = append(block.states, state)
		}
	}
	total := 0
	for _, block := range blocks {
		forEachNetworkBlockState(block.states, func(map[string]any) bool {
			total++
			return total <= maxNetworkBlockStates
		})
		if total > maxNetworkBlockStates {
			return fmt.Errorf("block palette states exceed limit of %d", maxNetworkBlockStates)
		}
	}
	scratch := make([]byte, 0, 0xff)
	for _, block := range blocks {
		var stateErr error
		forEachNetworkBlockState(block.states, func(properties map[string]any) bool {
			scratch, stateErr = registry.appendBlockState(BlockState{Name: block.name, Properties: properties}, scratch, true)
			return stateErr == nil
		})
		if stateErr != nil {
			return fmt.Errorf("block %s: %w", block.name, stateErr)
		}
	}
	return nil
}

// NetworkBlockEntries returns the entries of a StartGame block palette the client keeps, in the
// order their names first appear: an entry needs a vanilla_block_data compound, and a later entry
// replaces an earlier one with the same name.
func NetworkBlockEntries(entries []protocol.BlockEntry) []protocol.BlockEntry {
	kept := make([]protocol.BlockEntry, 0, len(entries))
	index := make(map[string]int, len(entries))
	for _, entry := range entries {
		if _, ok := entry.Properties["vanilla_block_data"].(map[string]any); !ok {
			continue
		}
		if at, ok := index[entry.Name]; ok {
			kept[at] = entry
			continue
		}
		index[entry.Name] = len(kept)
		kept = append(kept, entry)
	}
	return kept
}

// digestNetworkBlocks returns the definitions of the entries of a StartGame palette the client keeps.
func digestNetworkBlocks(entries []protocol.BlockEntry) ([]networkBlockDefinition, error) {
	for _, entry := range entries {
		// The client's NBT reader has no long array tag, so it cannot read such a palette at all.
		if containsLongArray(reflect.ValueOf(entry.Properties)) {
			return nil, fmt.Errorf("block %s holds a long array, which the client cannot load", entry.Name)
		}
	}
	kept := NetworkBlockEntries(entries)
	definitions := make([]networkBlockDefinition, 0, len(kept))
	for _, entry := range kept {
		states, traits, err := networkBlockStates(entry.Properties)
		if err != nil {
			return nil, fmt.Errorf("block %s: %w", entry.Name, err)
		}
		definitions = append(definitions, networkBlockDefinition{
			name: networkBlockName(entry.Name), states: states, traits: traits, vanilla: networkVanillaName(entry.Name),
		})
	}
	return definitions, nil
}

// networkBlockStates returns the states of one block definition: the states of its known traits in
// the order the traits are listed, then its properties in list order. Every property is its own
// state, even one named like another; the later one then names the value in the state.
func networkBlockStates(definition map[string]any) ([]customblock.TraitState, map[string]struct{}, error) {
	var states []customblock.TraitState
	traitStates := make(map[string]struct{})
	seenTraits := make(map[string]struct{})
	for _, trait := range networkCompounds(definition["traits"]) {
		name, _ := trait["name"].(string)
		// The first trait listed under a name wins.
		if _, seen := seenTraits[name]; seen {
			continue
		}
		seenTraits[name] = struct{}{}
		for _, state := range customblock.NetworkTraitStates(asciiLower(name), trait) {
			// The client ignores a trait state a block already has.
			if _, ok := traitStates[state.Name]; ok {
				continue
			}
			traitStates[state.Name] = struct{}{}
			states = append(states, state)
		}
	}

	for _, property := range networkCompounds(definition["properties"]) {
		name, _ := property["name"].(string)
		values, ok := networkListValues(property["enum"])
		if !ok {
			continue
		}
		if len(values) == 0 {
			return nil, nil, fmt.Errorf("property %q has no values, which the client cannot load", name)
		}
		states = append(states, customblock.TraitState{Name: name, Values: values})
	}
	return states, traitStates, nil
}

// networkCompounds returns the compound elements of an NBT list, skipping every other element as
// the client does. A value that is not a list has no elements.
func networkCompounds(value any) []map[string]any {
	elements, ok := networkListValues(value)
	if !ok {
		return nil
	}
	compounds := make([]map[string]any, 0, len(elements))
	for _, element := range elements {
		if compound, ok := element.(map[string]any); ok {
			compounds = append(compounds, compound)
		}
	}
	return compounds
}

// networkListValues returns the elements of a decoded NBT list of any element type, keeping each
// element's own type, or false if value is not a list.
func networkListValues(value any) ([]any, bool) {
	if value == nil {
		return nil, false
	}
	list := reflect.ValueOf(value)
	if list.Kind() != reflect.Slice {
		return nil, false
	}
	values := make([]any, list.Len())
	for i := range values {
		values[i] = list.Index(i).Interface()
	}
	return values, true
}

// forEachNetworkBlockState yields the state properties of a block in the client's runtime ID order.
// Each state takes a bit field of the packed data value, in the order the states were added, wide
// enough for its values. The client walks every data value of at most 16 bits in ascending order
// and skips those a state's mask marks out of range, then reads each state's value from the data
// value. Both use the client's 16-bit masks with shift counts taken modulo 32, so a field that does
// not fit below bit 16 is never out of range and reads only the bits it has, or none.
func forEachNetworkBlockState(states []customblock.TraitState, yield func(map[string]any) bool) {
	type field struct{ start, width, mask, read uint32 }
	fields := make([]field, len(states))
	var offset, bits uint32
	for i, state := range states {
		width := uint32(networkStateBitWidth(len(state.Values)))
		mask := (uint32(0xffff) << ((16 - width) & 31) & 0xffff) >> ((16 - (offset + width)) & 31)
		fields[i] = field{start: offset & 31, width: width, mask: mask, read: uint32(0xffff) >> ((16 - width) & 31)}
		offset += width
		bits += width
	}
	bits = min(bits, maxBlockStateBits)
	properties := make(map[string]any, len(states))
	for data := uint32(0); data < 1<<bits; data++ {
		valid := true
		for i, state := range states {
			count := uint32(max(len(state.Values), 1))
			if (fields[i].mask&data)>>fields[i].start >= count {
				valid = false
				break
			}
			if len(state.Values) == 0 {
				// The client counts a state without values as having one; it has nothing to name.
				continue
			}
			value := data >> fields[i].start & fields[i].read
			if value >= count {
				value = 0
			}
			properties[state.Name] = state.Values[value]
		}
		if !valid {
			continue
		}
		state := make(map[string]any, len(properties))
		for name, value := range properties {
			state[name] = value
		}
		if !yield(state) {
			return
		}
	}
}

// networkStateBitWidth returns the bits a state with count values occupies: none for a single
// value, otherwise enough to hold its largest index.
func networkStateBitWidth(count int) int {
	width := 0
	for count--; count > 0; count >>= 1 {
		width++
	}
	return width
}

// containsLongArray reports whether a decoded NBT value holds a long array anywhere.
func containsLongArray(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Interface:
		return !v.IsNil() && containsLongArray(v.Elem())
	case reflect.Array:
		if v.Type().Elem().Kind() == reflect.Int64 {
			return true
		}
		fallthrough
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if containsLongArray(v.Index(i)) {
				return true
			}
		}
	case reflect.Map:
		for it := v.MapRange(); it.Next(); {
			if containsLongArray(it.Value()) {
				return true
			}
		}
	}
	return false
}

// networkBlockName returns the name the client gives the block of a definition: a name without a
// namespace is in the minecraft namespace.
func networkBlockName(name string) string {
	if !strings.Contains(name, ":") {
		return "minecraft:" + name
	}
	return name
}

// networkVanillaName reports whether the client reads an entry name as an identifier in the
// minecraft namespace: one namespace and one name, neither empty, without whitespace, and a
// namespace of minecraft in any case.
func networkVanillaName(name string) bool {
	namespace, path, ok := strings.Cut(asciiLower(name), ":")
	if !ok || namespace == "" || path == "" || strings.Contains(path, ":") || strings.ContainsAny(name, " \t\r\n") {
		return false
	}
	return namespace == "minecraft"
}
