package block

import (
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestSmelter_ContainerSlotSourceID(t *testing.T) {
	for _, smelter := range []struct {
		name   string
		source ContainerSlotSource
		input  byte
	}{
		{"furnace", Furnace{}, protocol.ContainerFurnaceIngredient},
		{"blast furnace", BlastFurnace{}, protocol.ContainerBlastFurnaceIngredient},
		{"smoker", Smoker{}, protocol.ContainerSmokerIngredient},
	} {
		for _, test := range []struct {
			slot int
			id   byte
			want bool
		}{
			{-1, 0, false},
			{SmelterInputSlot, smelter.input, true},
			{SmelterFuelSlot, protocol.ContainerFurnaceFuel, true},
			{SmelterResultSlot, protocol.ContainerFurnaceResult, true},
			{3, 0, false},
		} {
			id, ok := smelter.source.ContainerSlotSourceID(test.slot)
			if id != test.id || ok != test.want {
				t.Errorf("%s slot %d source = %d, %v; want %d, %v", smelter.name, test.slot, id, ok, test.id, test.want)
			}
		}
	}
}
