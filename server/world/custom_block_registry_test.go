package world

import (
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestAddCustomBlocksPreservesVanillaRuntimeIDs(t *testing.T) {
	DefaultBlockRegistry.Finalize()
	registry := DefaultBlockRegistry.Clone()
	baseCount := registry.BlockCount()
	airRID := registry.AirRuntimeID()

	err := registry.AddCustomBlocks([]protocol.BlockEntry{{
		Name: "dragonfly:test_block",
		Properties: map[string]any{
			"properties": []any{
				map[string]any{
					"name": "dragonfly:variant",
					"enum": []any{int32(0), int32(1)},
				},
			},
		},
	}})
	if err != nil {
		t.Fatalf("AddCustomBlocks() error = %v", err)
	}

	if got := registry.AirRuntimeID(); got != airRID {
		t.Fatalf("expected air runtime ID %d to be preserved, got %d", airRID, got)
	}
	rid, ok := registry.StateToRuntimeID("dragonfly:test_block", map[string]any{"dragonfly:variant": int32(0)})
	if !ok {
		t.Fatal("expected custom block runtime ID")
	}
	if rid != uint32(baseCount) {
		t.Fatalf("expected first custom runtime ID %d, got %d", baseCount, rid)
	}
	if got := registry.BlockCount(); got != baseCount+2 {
		t.Fatalf("expected block count %d, got %d", baseCount+2, got)
	}
}

func TestAddCustomBlocksSkipsDuplicateStates(t *testing.T) {
	DefaultBlockRegistry.Finalize()
	registry := DefaultBlockRegistry.Clone()

	entries := []protocol.BlockEntry{{
		Name:       "dragonfly:plain_block",
		Properties: map[string]any{},
	}}
	if err := registry.AddCustomBlocks(entries); err != nil {
		t.Fatalf("AddCustomBlocks() first error = %v", err)
	}
	count := registry.BlockCount()
	if err := registry.AddCustomBlocks(entries); err != nil {
		t.Fatalf("AddCustomBlocks() second error = %v", err)
	}
	if got := registry.BlockCount(); got != count {
		t.Fatalf("expected duplicate state to be skipped, count %d -> %d", count, got)
	}
}

func TestNewCustomBlockRegistryPreservesVanillaRuntimeIDs(t *testing.T) {
	DefaultBlockRegistry.Finalize()
	airRID := DefaultBlockRegistry.AirRuntimeID()

	registry, err := NewCustomBlockRegistry([]protocol.BlockEntry{{
		Name:       "dragonfly:plain_block",
		Properties: map[string]any{},
	}})
	if err != nil {
		t.Fatalf("NewCustomBlockRegistry() error = %v", err)
	}
	if got := registry.AirRuntimeID(); got != airRID {
		t.Fatalf("expected air runtime ID %d to be preserved, got %d", airRID, got)
	}
}
