package item_test

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

// TestHasUseAction preserves native item operations when a consumer handles
// otherwise unused clicks. Ordinary weapons and an absent item have no use.
func TestHasUseAction(t *testing.T) {
	for _, tt := range []struct {
		name string
		it   world.Item
		want bool
	}{
		{"empty", nil, false},
		{"sword", item.Sword{}, false},
		{"stone", block.Stone{}, true},
		{"bucket", item.Bucket{}, true},
		{"egg", item.Egg{}, true},
		{"food", item.Apple{}, true},
		{"bow", item.Bow{}, true},
		{"crossbow", item.Crossbow{}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := item.HasUseAction(tt.it); got != tt.want {
				t.Fatalf("HasUseAction(%T) = %v, want %v", tt.it, got, tt.want)
			}
		})
	}
}
