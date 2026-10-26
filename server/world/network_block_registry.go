package world

import (
	"slices"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// NewNetworkBlockRegistry builds a finalized registry for a StartGame block palette.
// Vanilla data-driven blocks participate only when advertised, while built-in
// vanilla blocks always participate. Custom states sort alongside those blocks.
// The returned registry is independent of DefaultBlockRegistry.
func NewNetworkBlockRegistry(entries []protocol.BlockEntry) (BlockRegistry, error) {
	registry := NewBlockRegistry().(*BasicBlockRegistry)
	advertised := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		advertised[entry.Name] = struct{}{}
	}
	omitted := make(map[string]struct{})
	for _, block := range DataDrivenBlocks() {
		if _, ok := advertised[block.Name]; !ok {
			omitted[block.Name] = struct{}{}
			delete(registry.blockProperties, block.Name)
		}
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
	if err := AddCustomBlocks(registry, entries); err != nil {
		return nil, err
	}
	registry.Finalize()
	return registry, nil
}
