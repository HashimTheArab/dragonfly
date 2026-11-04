package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestContainerSource_OrdinaryStorageOnly(t *testing.T) {
	for _, test := range []struct {
		name  string
		block world.Block
		id    byte
		want  bool
	}{
		{name: "chest", block: Chest{}, id: protocol.ContainerLevelEntity, want: true},
		{name: "barrel", block: Barrel{}, id: protocol.ContainerBarrel, want: true},
		{name: "shulker", block: ShulkerBox{}, id: protocol.ContainerShulkerBox, want: true},
		{name: "furnace", block: Furnace{}},
		{name: "brewing stand", block: BrewingStand{}},
		{name: "non-storage", block: Stone{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, ok := test.block.(ContainerSource)
			if ok != test.want {
				t.Fatalf("ContainerSource implemented = %t, want %t", ok, test.want)
			}
			if ok && source.ContainerSourceID() != test.id {
				t.Fatalf("ContainerSourceID() = %d, want %d", source.ContainerSourceID(), test.id)
			}
		})
	}
}
