package world

import (
	"maps"
	"slices"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// NewNetworkBlockRegistry builds a finalized registry for a StartGame block palette.
// Vanilla data-driven blocks participate only when advertised, while built-in
// vanilla blocks always participate. Advertised custom implementations are
// retained; other custom states come from the advertised definitions.
// The returned registry is independent of DefaultBlockRegistry.
func NewNetworkBlockRegistry(entries []protocol.BlockEntry) (BlockRegistry, error) {
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
	for _, entry := range entries {
		if _, registered := registry.customBlocks[entry.Name]; registered || strings.HasPrefix(entry.Name, "minecraft:") {
			delete(omitted, entry.Name)
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
	entries, salts := unsaltedEntries(entries)
	if err := AddCustomBlocks(registry, entries); err != nil {
		return nil, err
	}
	registry.Finalize()
	registry.mu.Lock()
	registry.restoreNetworkHashSaltsLocked(salts)
	registry.mu.Unlock()
	return registry, nil
}

// unsaltedEntries returns entries without the hidden NetworkHashSaltProperty a hashed-ID server declares, so salted
// states match installed custom states, along with each salted block's salt.
func unsaltedEntries(entries []protocol.BlockEntry) ([]protocol.BlockEntry, map[string]int32) {
	var salts map[string]int32
	out := entries
	for i, entry := range entries {
		properties := entryProperties(entry)
		j := slices.IndexFunc(properties, func(p map[string]any) bool { return p["name"] == NetworkHashSaltProperty })
		if j < 0 {
			continue
		}
		values, _ := properties[j]["enum"].([]any)
		if len(values) != 1 {
			continue
		}
		salt, ok := values[0].(int32)
		if !ok {
			continue
		}
		if salts == nil {
			salts, out = make(map[string]int32), slices.Clone(entries)
		}
		salts[entry.Name] = salt
		entry.Properties = maps.Clone(entry.Properties)
		entry.Properties["properties"] = slices.Delete(properties, j, j+1)
		out[i] = entry
	}
	return out, salts
}

// entryProperties returns a copy of the property definitions of entry, whether built by a server or decoded from
// the network.
func entryProperties(entry protocol.BlockEntry) []map[string]any {
	switch properties := entry.Properties["properties"].(type) {
	case []map[string]any:
		return slices.Clone(properties)
	case []any:
		out := make([]map[string]any, 0, len(properties))
		for _, p := range properties {
			if m, ok := p.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}
