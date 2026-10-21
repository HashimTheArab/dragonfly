package recipe

import (
	"testing"

	"github.com/df-mc/dragonfly/server/item"
)

// A caller decoding recipes off the network can present anything, so the exported form
// reports an unknown pair rather than panicking the way the internal one does.
func TestMatchingStacks_ReportsUnknownItemsInsteadOfPanicking(t *testing.T) {
	type foreign struct{ Item }

	if _, ok := MatchingStacks(foreign{}, foreign{}); ok {
		t.Fatal("accepted a pair of unknown recipe items")
	}
	if _, ok := MatchingStacks(item.NewStack(item.Sword{Tier: item.ToolTierDiamond}, 1), foreign{}); ok {
		t.Fatal("accepted an unknown expected item")
	}
	match, ok := MatchingStacks(
		item.NewStack(item.Sword{Tier: item.ToolTierDiamond}, 1),
		item.NewStack(item.Sword{Tier: item.ToolTierDiamond}, 1),
	)
	if !ok || !match {
		t.Fatalf("identical stacks did not match: match=%v ok=%v", match, ok)
	}
}

func TestMatchingStacks_EmptyAndMetadataSemantics(t *testing.T) {
	exact := gridStack("minecraft:oak_planks", 0, 1)
	variant := gridStack("minecraft:oak_planks", 1, 1)
	wildcard := exact.WithValue("variants", true)
	tag := NewItemTag("minecraft:planks", 1)
	for _, tt := range []struct {
		has, expected Item
		want          bool
	}{
		{item.Stack{}, item.Stack{}, true},
		{item.Stack{}, wildcard, false},
		{item.Stack{}, tag, false},
		{tag, item.Stack{}, false},
		{variant, exact, false},
		{variant, wildcard, true},
		{variant, tag, true},
		{tag, exact, true},
		{tag, tag, true},
		{tag, NewItemTag("minecraft:is_sword", 1), false},
	} {
		match, ok := MatchingStacks(tt.has, tt.expected)
		if !ok || match != tt.want {
			t.Fatalf("MatchingStacks(%v,%v) = %v,%v, want %v,true", tt.has, tt.expected, match, ok, tt.want)
		}
	}
}
