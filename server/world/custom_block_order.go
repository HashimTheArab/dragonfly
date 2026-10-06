package world

import (
	"maps"
	"slices"
	"sort"

	"github.com/df-mc/dragonfly/server/block/customblock"
)

// sortForNetworkLocked orders the blocks as the client orders its block palette: by the network
// hash of their names, and the states of one custom block by the client's permutation index. The
// registry lock must be held.
func (br *BasicBlockRegistry) sortForNetworkLocked() {
	type entry struct {
		block Block
		name  string
		hash  uint64
		index int
	}
	axes := make(map[string][]customblock.TraitState, len(br.customBlocks))
	for name, b := range br.customBlocks {
		axes[name] = customBlockStateAxes(b)
	}
	entries := make([]entry, len(br.blocks))
	for i, b := range br.blocks {
		name, properties := b.EncodeBlock()
		index := -1
		if a, ok := axes[name]; ok {
			index = customBlockStateIndex(a, properties)
		}
		entries[i] = entry{block: b, name: name, hash: NetworkBlockHash(name), index: index}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].hash != entries[j].hash {
			return entries[i].hash < entries[j].hash
		}
		return entries[i].name == entries[j].name && entries[i].index < entries[j].index
	})
	for i, e := range entries {
		br.blocks[i] = e.block
	}
}

// validateCustomBlock returns an error if the traits of a custom block are invalid.
func validateCustomBlock(b CustomBlock) error {
	if traited, ok := b.(interface{ Traits() []customblock.Trait }); ok {
		_, err := customblock.SortTraits(traited.Traits())
		return err
	}
	return nil
}

// customBlockStateAxes returns the states of a custom block in the order the client adds them to
// the block: the states of its traits in vanilla's trait order, then its properties in the order
// of the properties list, which dragonfly sends sorted by name.
func customBlockStateAxes(b CustomBlock) []customblock.TraitState {
	var axes []customblock.TraitState
	if traited, ok := b.(interface{ Traits() []customblock.Trait }); ok {
		// RegisterBlock has already rejected invalid traits.
		traits, _ := customblock.SortTraits(traited.Traits())
		for _, trait := range traits {
			axes = append(axes, trait.States()...)
		}
	}
	if permutable, ok := b.(interface{ States() map[string][]any }); ok {
		states := permutable.States()
		for _, name := range slices.Sorted(maps.Keys(states)) {
			axes = append(axes, customblock.TraitState{Name: name, Values: states[name]})
		}
	}
	return axes
}

// customBlockStateIndex returns the client's permutation index of a state of a custom block with
// the axes passed, in which the first state added varies fastest, or -1 if a value is unknown.
func customBlockStateIndex(axes []customblock.TraitState, properties map[string]any) int {
	index, radix := 0, 1
	for _, axis := range axes {
		value := stateValueKey(properties[axis.Name])
		i := slices.IndexFunc(axis.Values, func(v any) bool { return stateValueKey(v) == value })
		if i < 0 {
			return -1
		}
		index += i * radix
		radix *= len(axis.Values)
	}
	return index
}

// stateValueKey returns v in a form equal for every type a state value may be declared or encoded
// as: a bool or integer of any width becomes an int64.
func stateValueKey(v any) any {
	switch v := v.(type) {
	case bool:
		if v {
			return int64(1)
		}
		return int64(0)
	case uint8:
		return int64(v)
	case int8:
		return int64(v)
	case uint16:
		return int64(v)
	case int16:
		return int64(v)
	case uint32:
		return int64(v)
	case int32:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	}
	return v
}
