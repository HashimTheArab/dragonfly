package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world"
)

func TestSapling_SupportedByDirtAndMudBlocks(t *testing.T) {
	t.Parallel()

	sapling := Sapling{Wood: OakWood()}
	for name, support := range map[string]world.Block{
		"dirt":                 Dirt{},
		"coarse dirt":          Dirt{Coarse: true},
		"grass":                Grass{},
		"podzol":               Podzol{},
		"farmland":             Farmland{},
		"mud":                  Mud{},
		"muddy mangrove roots": MuddyMangroveRoots{},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if !supportsVegetation(sapling, support) {
				t.Fatalf("%T does not support saplings", support)
			}
		})
	}
}
