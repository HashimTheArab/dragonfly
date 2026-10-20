package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestBrewingStand_ContainerSlotSourceID(t *testing.T) {
	stand := BrewingStand{}
	for _, test := range []struct {
		slot int
		id   byte
		want bool
	}{
		{-1, 0, false},
		{0, protocol.ContainerBrewingStandInput, true},
		{1, protocol.ContainerBrewingStandResult, true},
		{2, protocol.ContainerBrewingStandResult, true},
		{3, protocol.ContainerBrewingStandResult, true},
		{4, protocol.ContainerBrewingStandFuel, true},
		{5, 0, false},
		{255, 0, false},
	} {
		id, ok := stand.ContainerSlotSourceID(test.slot)
		if id != test.id || ok != test.want {
			t.Errorf("slot %d source = %d, %v; want %d, %v", test.slot, id, ok, test.id, test.want)
		}
	}
	if _, ok := stand.FuelItem().(item.BlazePowder); !ok {
		t.Fatalf("FuelItem = %T, want BlazePowder", stand.FuelItem())
	}
}
