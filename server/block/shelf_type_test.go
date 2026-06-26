package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world"
)

func TestBambooShelfItemHashMatchesRegisteredBlockState(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()

	b := Shelf{Wood: BambooWood(), Bamboo: true}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("bamboo shelf item did not resolve to a registered runtime ID: %v", r)
		}
	}()
	_ = world.DefaultBlockRegistry.BlockRuntimeID(b)
}
