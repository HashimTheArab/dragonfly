package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
)

func TestShulkerBox_StorageAdmission(t *testing.T) {
	for _, tc := range []struct {
		name    string
		stack   item.Stack
		allowed bool
	}{
		{"empty", item.Stack{}, true},
		{"ordinary item", item.NewStack(item.Diamond{}, 1), true},
		{"nested shulker", item.NewStack(NewShulkerBox(), 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanStoreInShulkerBox(tc.stack, 0); got != tc.allowed {
				t.Fatalf("admission = %v, want %v", got, tc.allowed)
			}
			inv := NewShulkerBox().Inventory(nil, cube.Pos{})
			if err := inv.SetItem(0, tc.stack); err != nil {
				t.Fatal(err)
			}
			stored, err := inv.Item(0)
			if err != nil {
				t.Fatal(err)
			}
			if tc.allowed && !stored.Equal(tc.stack) || !tc.allowed && !stored.Empty() {
				t.Fatal("inventory validator disagrees with exported storage rule")
			}
		})
	}
}
