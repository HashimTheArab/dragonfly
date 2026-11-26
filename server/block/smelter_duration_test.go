package block

import (
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/item"
)

func TestSmelter_FuelSmeltsVanillaItemCount(t *testing.T) {
	planks := Planks{Wood: OakWood()}.FuelInfo()
	coal := item.Coal{}.FuelInfo()
	for _, test := range []struct {
		name    string
		smelter interface {
			CookDuration() time.Duration
			ItemsPerFuel(item.FuelInfo) float64
		}
		cook time.Duration
	}{
		{"furnace", NewFurnace(0), time.Second * 10},
		{"blast furnace", NewBlastFurnace(0), time.Second * 5},
		{"smoker", NewSmoker(0), time.Second * 5},
	} {
		if got := test.smelter.CookDuration(); got != test.cook {
			t.Errorf("%s cook duration = %v, want %v", test.name, got, test.cook)
		}
		if got := test.smelter.ItemsPerFuel(planks); got != 1.5 {
			t.Errorf("%s items per plank = %v, want 1.5", test.name, got)
		}
		if got := test.smelter.ItemsPerFuel(coal); got != 8 {
			t.Errorf("%s items per coal = %v, want 8", test.name, got)
		}

		s := newSmelter()
		_ = s.inventory.SetItem(SmelterInputSlot, item.NewStack(item.RawIron{}, 64))
		_ = s.inventory.SetItem(SmelterFuelSlot, item.NewStack(item.Coal{}, 1))
		for range 2000 {
			s.tickSmelting(test.smelter.CookDuration(), time.Millisecond*100, true, func(item.SmeltInfo) bool { return true })
		}
		if result, _ := s.inventory.Item(SmelterResultSlot); result.Count() != 8 {
			t.Errorf("%s smelted %d items with one coal, want 8", test.name, result.Count())
		}
	}
}
