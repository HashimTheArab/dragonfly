package blockinternal

import (
	"reflect"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/customblock"
)

// A block with selection disabled sends a disabled selection box, whatever box it also has; a
// permutation's box still applies; and a block without either sends none, which the client
// reads as the full cube.
func TestSelectionDisabledSendsADisabledSelectionBox(t *testing.T) {
	box := cube.Box(0, 0, 0, 1, 1, 0.125)
	disabled := componentsFromProperties("test:plate", customblock.Properties{
		Cube: true, SelectionBox: box, SelectionDisabled: true,
	})
	if got, want := disabled["minecraft:selection_box"], map[string]any{"enabled": false}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selection_box = %#v, want %#v", got, want)
	}
	permutation := componentsFromProperties("test:plate", customblock.Properties{SelectionBox: box})
	if got := permutation["minecraft:selection_box"].(map[string]any)["enabled"]; got != true {
		t.Fatalf("a permutation's box sends enabled %v", got)
	}
	if _, ok := componentsFromProperties("test:plate", customblock.Properties{Cube: true})["minecraft:selection_box"]; ok {
		t.Fatal("a block without a box or a disabled selection sends a selection box")
	}
}
