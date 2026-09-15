package creative

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world"
)

func TestVanillaItems(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	groups, items := VanillaItems()
	if len(groups) == 0 || len(items) == 0 {
		t.Fatalf("empty vanilla catalog: %d groups, %d items", len(groups), len(items))
	}
	categories := make(map[string]Category, len(groups))
	for _, group := range groups {
		categories[group.Name] = group.Category
	}
	for _, entry := range items {
		block, ok := entry.Stack.Item().(world.Block)
		if !ok {
			continue
		}
		name, _ := block.EncodeBlock()
		if name == "minecraft:diamond_ore" {
			if got := categories[entry.Group]; got != NatureCategory() {
				t.Fatalf("diamond ore category = %d, want nature", got.Uint8())
			}
			return
		}
	}
	t.Fatal("diamond ore missing from vanilla creative catalog")
}
