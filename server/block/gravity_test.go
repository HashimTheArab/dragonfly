package block_test

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
)

type embeddedSand struct{ block.Sand }

// TestBlock_GravityClassificationFollowsBehaviour covers values, pointers and
// wrappers that inherit gravity, rather than maintaining a concrete-type list.
func TestBlock_GravityClassificationFollowsBehaviour(t *testing.T) {
	for _, test := range []struct {
		name    string
		block   world.Block
		falling bool
	}{
		{"sand", block.Sand{}, true},
		{"gravel pointer", &block.Gravel{}, true},
		{"powder", block.ConcretePowder{}, true},
		{"anvil", block.Anvil{}, true},
		{"egg", block.DragonEgg{}, true},
		{"inherited behaviour", embeddedSand{}, true},
		{"stone", block.Stone{}, false},
		{"air", block.Air{}, false},
		{"nil", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := block.IsGravityAffected(test.block); got != test.falling {
				t.Fatalf("IsGravityAffected(%T) = %v, want %v", test.block, got, test.falling)
			}
		})
	}
}
