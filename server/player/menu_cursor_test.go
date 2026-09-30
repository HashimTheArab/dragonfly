package player

import (
	"testing"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
)

func TestClearCursorItem_PreservesUnrelatedItems(t *testing.T) {
	button := item.NewStack(item.Diamond{}, 1).WithCustomName("Confirm")
	for _, test := range []struct {
		name   string
		cursor item.Stack
		clear  bool
	}{
		{"expected menu item", button, true},
		{"different name", button.WithCustomName("Player's diamond"), false},
		{"different count", button.Grow(1), false},
		{"empty cursor", item.Stack{}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := &Player{playerData: &playerData{ui: inventory.New(54, nil)}}
			_ = p.ui.SetItem(0, test.cursor)
			if got := p.ClearCursorItem(button); got != test.clear {
				t.Fatalf("cleared = %v, want %v", got, test.clear)
			}
			got, _ := p.ui.Item(0)
			if test.clear && !got.Empty() {
				t.Fatal("accepted menu item remains on cursor")
			}
			if !test.clear && !got.Equal(test.cursor) {
				t.Fatal("unrelated cursor item changed")
			}
		})
	}
}
