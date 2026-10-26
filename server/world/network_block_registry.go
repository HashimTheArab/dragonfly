package world

import (
	"reflect"
	"slices"
	"strings"

	"github.com/df-mc/dragonfly/server/block/customblock"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// NewNetworkBlockRegistry builds a finalized registry for a StartGame block palette, read as the
// client reads it. Built-in vanilla blocks always participate. Vanilla data-driven blocks and
// custom blocks participate only when the palette defines them; a registered custom
// implementation is kept only when its states match the definition, and other definitions add
// the states the client derives from them. The returned registry is independent of
// DefaultBlockRegistry.
func NewNetworkBlockRegistry(entries []protocol.BlockEntry) (BlockRegistry, error) {
	definitions, err := digestNetworkBlocks(entries)
	if err != nil {
		return nil, err
	}
	defined := make(map[string]networkBlockDefinition, len(definitions))
	for _, definition := range definitions {
		defined[definition.name] = definition
	}

	registry := NewBlockRegistry().(*BasicBlockRegistry)
	omitted := make(map[string]struct{}, len(DataDrivenBlocks()))
	for _, block := range DataDrivenBlocks() {
		omitted[block.Name] = struct{}{}
	}
	for _, block := range registry.blocks {
		name, _ := block.EncodeBlock()
		if !vanillaNamespace(name) {
			omitted[name] = struct{}{}
		}
	}
	for name := range defined {
		if implementation, registered := registry.customBlocks[name]; registered {
			if networkStatesEqual(customBlockStateAxes(implementation), defined[name].states) {
				delete(omitted, name)
			}
			continue
		}
		if vanillaNamespace(name) {
			delete(omitted, name)
		}
	}
	for name := range omitted {
		delete(registry.blockProperties, name)
		delete(registry.customBlocks, name)
	}
	registry.blocks = slices.DeleteFunc(registry.blocks, func(block Block) bool {
		name, _ := block.EncodeBlock()
		_, skip := omitted[name]
		return skip
	})
	// Custom-state insertion uses this index before finalization rebuilds it.
	clear(registry.stateRuntimeIDs)
	for rid, block := range registry.blocks {
		name, properties := block.EncodeBlock()
		registry.stateRuntimeIDs[stateHash{name: name, properties: hashProperties(properties)}] = uint32(rid)
	}
	if err := addNetworkBlocks(registry, definitions); err != nil {
		return nil, err
	}
	registry.Finalize()
	return registry, nil
}

// networkStatesEqual reports whether an implementation's states are a definition's: the same states
// with values of the same tag types, in the same order, within the client's 16-bit state data.
func networkStatesEqual(implementation, definition []customblock.TraitState) bool {
	bits := 0
	for _, state := range implementation {
		bits += networkStateBitWidth(len(state.Values))
	}
	return bits <= maxBlockStateBits && slices.EqualFunc(implementation, definition, func(x, y customblock.TraitState) bool {
		return x.Name == y.Name && slices.EqualFunc(x.Values, y.Values, func(v, w any) bool {
			return reflect.DeepEqual(networkTagValue(v), networkTagValue(w))
		})
	})
}

// networkTagValue returns v as the value of its network tag: a bool is a byte.
func networkTagValue(v any) any {
	if b, ok := v.(bool); ok {
		if b {
			return uint8(1)
		}
		return uint8(0)
	}
	return v
}

// vanillaNamespace reports whether name is in the minecraft namespace, ignoring case as the client does.
func vanillaNamespace(name string) bool {
	return strings.HasPrefix(asciiLower(name), "minecraft:")
}
