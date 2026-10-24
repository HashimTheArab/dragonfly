package session

import (
	"reflect"
	"testing"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/item/recipe"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestHandleCraft_ValidatesWholeGridBeforeConsumption(t *testing.T) {
	apple, bread := item.NewStack(item.Apple{}, 1), item.NewStack(item.Bread{}, 1)
	output := item.NewStack(item.Stick{}, 1)
	for _, large := range []bool{false, true} {
		for _, tt := range []struct {
			name  string
			cells map[int]item.Stack
			ok    bool
		}{
			{"mirrored", map[int]item.Stack{0: bread.Grow(2), 1: apple.Grow(2)}, true},
			{"missing ingredient", map[int]item.Stack{0: apple.Grow(2)}, false},
			{"wrong shape", map[int]item.Stack{0: apple.Grow(2), 2: bread.Grow(2)}, false},
			{"extra item", map[int]item.Stack{0: apple.Grow(2), 1: bread.Grow(2), 2: apple}, false},
		} {
			t.Run(tt.name+map[bool]string{false: "/small", true: "/large"}[large], func(t *testing.T) {
				s := &Session{ui: inventory.New(54, nil), recipes: map[uint32]recipe.Recipe{1: recipe.NewShaped([]recipe.Item{apple, bread}, output, recipe.NewShape(2, 1), "crafting_table")}}
				if large {
					s.openedContainerID.Store(1)
				}
				offset := int(s.craftingOffset())
				for slot, st := range tt.cells {
					if err := s.ui.SetItem(offset+slot, st); err != nil {
						t.Fatal(err)
					}
				}
				before := s.ui.Slots()
				h := &ItemStackRequestHandler{changes: make(map[byte]map[byte]changeInfo), responseChanges: make(map[int32]map[*inventory.Inventory]map[byte]responseChange)}
				err := h.handleCraft(&protocol.CraftRecipeStackRequestAction{RecipeNetworkID: 1, NumberOfCrafts: 2}, s, nil)
				if (err == nil) != tt.ok {
					t.Fatalf("err=%v, want success=%v", err, tt.ok)
				}
				if !tt.ok {
					if !reflect.DeepEqual(before, s.ui.Slots()) || len(h.changes) != 0 {
						t.Fatal("failed match mutated inventory")
					}
					return
				}
				for slot := range tt.cells {
					st, _ := s.ui.Item(offset + slot)
					if st.Count() != 1 {
						t.Fatalf("slot %d remaining count=%d", slot, st.Count())
					}
				}
				st, _ := s.ui.Item(CraftingResult)
				if !st.Comparable(output) || st.Count() != 2 {
					t.Fatalf("unexpected output %v", st)
				}
			})
		}
	}
}
