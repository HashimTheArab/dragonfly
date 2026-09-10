package world

import (
	"bytes"
	"fmt"
	"maps"

	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
)

// MarshalBlockRegistry records a finalized registry's state palette in runtime-ID
// order. NBT preserves the property types used for block-state identity. Block
// implementations are resolved by the decoder's installed Dragonfly version.
func MarshalBlockRegistry(registry BlockRegistry) ([]byte, error) {
	if basic, ok := registry.(*BasicBlockRegistry); ok {
		registry = basic.Clone()
	}
	states := make([]BlockState, registry.BlockCount())
	for i := range states {
		name, properties, ok := registry.RuntimeIDToState(uint32(i))
		if !ok {
			return nil, fmt.Errorf("missing block state for runtime ID %d", i)
		}
		states[i] = BlockState{Name: name, Properties: properties, Version: chunk.CurrentBlockVersion}
	}
	var out bytes.Buffer
	enc := nbt.NewEncoder(&out)
	enc.SortMaps = true
	if err := enc.Encode(struct {
		States []BlockState `nbt:"states"`
	}{states}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// UnmarshalBlockRegistry restores a recorded state palette in its exact runtime-ID
// order. Known states use the installed Dragonfly block implementations; custom
// and historical states without an implementation remain unknown blocks.
func UnmarshalBlockRegistry(data []byte) (registry BlockRegistry, err error) {
	defer func() {
		if v := recover(); v != nil {
			registry, err = nil, fmt.Errorf("restore block registry: %v", v)
		}
	}()
	var snapshot struct {
		States []BlockState `nbt:"states"`
	}
	if err := nbt.NewDecoder(bytes.NewReader(data)).Decode(&snapshot); err != nil {
		return nil, err
	}
	if len(snapshot.States) == 0 {
		return nil, fmt.Errorf("empty block registry snapshot")
	}
	DefaultBlockRegistry.Finalize()
	r := &BasicBlockRegistry{
		blocks:          make([]Block, len(snapshot.States)),
		blockProperties: make(map[string]map[string]any),
		customBlocks:    make(map[string]CustomBlock),
	}
	for i, state := range snapshot.States {
		if !ValidBlockPropertyValues(state.Properties) {
			return nil, fmt.Errorf("invalid properties for %s", state.Name)
		}
		b, known := DefaultBlockRegistry.BlockByName(state.Name, state.Properties)
		if !known {
			b = unknownBlock{BlockState: state}
		} else if name, properties := b.EncodeBlock(); name != state.Name || !maps.EqualFunc(properties, state.Properties, snapshotPropertyEqual) {
			return nil, fmt.Errorf("block state does not match registered properties: %s", state.Name)
		}
		r.blocks[i] = b
		if _, present := r.blockProperties[state.Name]; !present {
			r.blockProperties[state.Name] = state.Properties
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finalizeLocked()
	return r, nil
}

// snapshotPropertyEqual compares palette scalars after NBT's bool-to-byte encoding.
func snapshotPropertyEqual(a, b any) bool {
	if value, ok := a.(bool); ok {
		a = uint8(0)
		if value {
			a = uint8(1)
		}
	}
	if value, ok := b.(bool); ok {
		b = uint8(0)
		if value {
			b = uint8(1)
		}
	}
	return a == b
}
