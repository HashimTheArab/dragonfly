package item_test

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/item"
)

func TestStack_HasDefaultData(t *testing.T) {
	for _, tt := range []struct {
		name  string
		stack item.Stack
		wire  map[string]any
		want  bool
	}{
		{"plain", item.NewStack(block.Stone{}, 64), nil, true},
		{"named", item.NewStack(block.Stone{}, 1).WithCustomName("Keep"), nil, false},
		{"unknown wire data", item.NewStack(block.Stone{}, 1), map[string]any{"private": int32(1)}, false},
		{"empty", item.Stack{}, nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stack.HasDefaultData(tt.wire); got != tt.want {
				t.Fatalf("HasDefaultData=%t, want %t", got, tt.want)
			}
		})
	}
}
