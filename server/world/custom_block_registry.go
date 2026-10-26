package world

import (
	"fmt"
	"reflect"
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

// addNetworkBlocks appends the states of every definition whose name registry does not hold yet.
func addNetworkBlocks(registry *BasicBlockRegistry, definitions []networkBlockDefinition) error {
	total := 0
	scratch := make([]byte, 0, 0xff)
	for _, definition := range definitions {
		if _, registered := registry.blockProperties[definition.name]; registered {
			continue
		}
		var stateErr error
		forEachNetworkBlockState(definition.states, func(properties map[string]any) bool {
			if total++; total > maxNetworkBlockStates {
				stateErr = fmt.Errorf("block palette states exceed limit of %d", maxNetworkBlockStates)
				return false
			}
			scratch, stateErr = registry.addCustomBlockState(BlockState{Name: definition.name, Properties: properties}, scratch)
			return stateErr == nil
		})
		if stateErr != nil {
			return fmt.Errorf("block %s: %w", definition.name, stateErr)
		}
	}
	return nil
}

// digestNetworkBlocks returns the definitions of a StartGame palette the client keeps, each under
// the last entry with its name, in the order their names first appear.
func digestNetworkBlocks(entries []protocol.BlockEntry) ([]networkBlockDefinition, error) {
	definitions := make([]networkBlockDefinition, 0, len(entries))
	index := make(map[string]int, len(entries))
	for _, entry := range entries {
		if _, ok := entry.Properties["vanilla_block_data"].(map[string]any); !ok {
			continue
		}
		states, err := networkBlockStates(entry.Properties)
		if err != nil {
			return nil, fmt.Errorf("block %s: %w", entry.Name, err)
		}
		definition := networkBlockDefinition{name: entry.Name, states: states}
		if at, ok := index[entry.Name]; ok {
			definitions[at] = definition
			continue
		}
		index[entry.Name] = len(definitions)
		definitions = append(definitions, definition)
	}
	return definitions, nil
}

// networkBlockStates returns the states of one block definition: the states of its known traits in
// the order the traits are listed, then its properties in list order.
func networkBlockStates(definition map[string]any) ([]customblock.TraitState, error) {
	var states []customblock.TraitState
	added := make(map[string]struct{})
	addState := func(state customblock.TraitState) {
		// The client ignores a state a block already has.
		if _, ok := added[state.Name]; ok {
			return
		}
		added[state.Name] = struct{}{}
		states = append(states, state)
	}

	seenTraits := make(map[string]struct{})
	for _, trait := range networkCompounds(definition["traits"]) {
		name, _ := trait["name"].(string)
		// The first trait listed under a name wins.
		if _, seen := seenTraits[name]; seen {
			continue
		}
		seenTraits[name] = struct{}{}
		for _, state := range customblock.NetworkTraitStates(strings.ToLower(name), trait) {
			addState(state)
		}
	}

	for _, property := range networkCompounds(definition["properties"]) {
		name, _ := property["name"].(string)
		values, ok := networkListValues(property["enum"])
		if !ok {
			continue
		}
		if len(values) == 0 {
			return nil, fmt.Errorf("property %q has no values, which the client cannot load", name)
		}
		if _, ok := added[name]; ok {
			return nil, fmt.Errorf("state %q is declared twice", name)
		}
		addState(customblock.TraitState{Name: name, Values: values})
	}
	return states, nil
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
// enough for its values; the client walks every data value of at most 16 bits in ascending order
// and skips those naming a value a state does not have.
func forEachNetworkBlockState(states []customblock.TraitState, yield func(map[string]any) bool) {
	starts := make([]int, len(states))
	widths := make([]int, len(states))
	bits := 0
	for i, state := range states {
		starts[i], widths[i] = bits, networkStateBitWidth(len(state.Values))
		bits += widths[i]
	}
	bits = min(bits, maxBlockStateBits)
	properties := make(map[string]any, len(states))
	for data := 0; data < 1<<bits; data++ {
		valid := true
		for i, state := range states {
			value := 0
			if widths[i] != 0 && starts[i] < maxBlockStateBits {
				mask := (1<<widths[i] - 1) << starts[i] & (1<<maxBlockStateBits - 1)
				value = (data & mask) >> starts[i]
			}
			if value >= max(len(state.Values), 1) {
				valid = false
				break
			}
			if len(state.Values) == 0 {
				// The client counts a state without values as having one; it has nothing to name.
				continue
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
