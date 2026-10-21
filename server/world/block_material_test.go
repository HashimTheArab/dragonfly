package world

import "testing"

// TestBlockRegistry_PlacementItemMetadata rejects ambiguous and missing states
// without replacing an existing registration's quantity.
func TestBlockRegistry_PlacementItemMetadata(t *testing.T) {
	registry := NewBlockRegistry()
	registry.RegisterBlockState(BlockState{Name: "example:material"})
	registry.RegisterBlock(testPlacementItem{name: "example:material", count: 3})
	for _, test := range []struct {
		name     string
		register func()
	}{
		{"missing state", func() { registry.RegisterBlock(testPlacementItem{name: "example:missing", count: 2}) }},
		{"duplicate state", func() { registry.RegisterBlock(testPlacementItem{name: "example:material", count: 2}) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected invalid registration to panic")
				}
			}()
			test.register()
		})
	}
	registry.Finalize()
	b, ok := registry.BlockByName("example:material", nil)
	if !ok || !IsUnknownBlock(b) {
		t.Fatal("metadata replaced unknown block behaviour")
	}
	it, count := PlacementItem(b)
	if it == nil || count != 3 {
		t.Fatalf("placement item = %v, count = %d", it, count)
	}
	if name, _ := it.EncodeItem(); name != "example:item" {
		t.Fatalf("item = %s", name)
	}
}

// testPlacementItem supplies inventory facts without world.Block behaviour.
type testPlacementItem struct {
	name  string
	count int
}

// EncodeBlock identifies the state receiving the metadata.
func (b testPlacementItem) EncodeBlock() (string, map[string]any) { return b.name, nil }

// EncodeItem supplies the corresponding inventory identity.
func (testPlacementItem) EncodeItem() (string, int16) { return "example:item", 0 }

// PlacementItemCount supplies the quantity stored in the test registration.
func (b testPlacementItem) PlacementItemCount() int { return b.count }
