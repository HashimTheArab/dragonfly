package item_test

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
)

func TestItem_HasUseBehaviour(t *testing.T) {
	for _, test := range []struct {
		name  string
		stack item.Stack
		want  bool
	}{
		{name: "empty"},
		{name: "empty block", stack: item.NewStack(block.Stone{}, 0)},
		{name: "ordinary item", stack: item.NewStack(item.Stick{}, 1)},
		{name: "weapon", stack: item.NewStack(item.Sword{Tier: item.ToolTierIron}, 1)},
		{name: "placement", stack: item.NewStack(block.Stone{}, 1), want: true},
		{name: "block use", stack: item.NewStack(item.FlintAndSteel{}, 1), want: true},
		{name: "air use", stack: item.NewStack(item.Snowball{}, 1), want: true},
		{name: "consumption", stack: item.NewStack(item.Apple{}, 1), want: true},
		{name: "charge", stack: item.NewStack(struct {
			item.Stick
			item.Chargeable
		}{}, 1), want: true},
		{name: "release", stack: item.NewStack(struct {
			item.Stick
			item.Releasable
		}{}, 1), want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := item.HasUseBehaviour(test.stack); got != test.want {
				t.Fatalf("HasUseBehaviour() = %v, want %v", got, test.want)
			}
		})
	}
}
