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
		if !strings.HasPrefix(name, "minecraft:") {
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
		if strings.HasPrefix(name, "minecraft:") {
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

// networkStatesEqual reports whether two blocks have the same states with the same values, in the
// same order.
func networkStatesEqual(a, b []customblock.TraitState) bool {
	return slices.EqualFunc(a, b, func(x, y customblock.TraitState) bool {
		return x.Name == y.Name && slices.EqualFunc(x.Values, y.Values, func(v, w any) bool {
			return reflect.DeepEqual(stateValueKey(v), stateValueKey(w))
		})
	})
}
