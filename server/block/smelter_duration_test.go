package block

import (
	"fmt"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/world"
)

type testSmelter interface {
	world.Block
	world.NBTer
	CookDuration() time.Duration
	ItemsPerFuel(item.FuelInfo) float64
	Durations() (remaining, maximum, cook time.Duration)
	Inventory(*world.Tx, cube.Pos) *inventory.Inventory
	AddViewer(ContainerViewer, *world.Tx, cube.Pos)
	Tick(int64, cube.Pos, *world.Tx)
}

type furnaceProgressViewer struct {
	world.NopViewer
	minCook time.Duration
}

func (*furnaceProgressViewer) ViewSlotChange(int, item.Stack) {}

func (v *furnaceProgressViewer) ViewFurnaceUpdate(_, cook, _, _, _, _ time.Duration) {
	v.minCook = min(v.minCook, cook)
}

type longFuel struct{}

func (longFuel) EncodeItem() (string, int16) { return "dragonfly:test_long_fuel", 0 }

func (longFuel) FuelInfo() item.FuelInfo { return item.FuelInfo{Duration: time.Hour * 24 * 11} }

type smelterCase struct {
	name  string
	build func() testSmelter
	cook  time.Duration
	input item.Stack
}

var smelterCases = []smelterCase{
	{"furnace", func() testSmelter { return NewFurnace(0) }, time.Second * 10, item.NewStack(item.RawIron{}, 64)},
	{"blast furnace", func() testSmelter { return NewBlastFurnace(0) }, time.Second * 5, item.NewStack(item.RawIron{}, 64)},
	{"smoker", func() testSmelter { return NewSmoker(0) }, time.Second * 5, item.NewStack(item.Beef{}, 64)},
}

func tickSmelterBlock(tx *world.Tx, pos cube.Pos, ticks int) {
	for range ticks {
		tx.Block(pos).(testSmelter).Tick(0, pos, tx)
	}
}

func TestSmelter_FuelSmeltsVanillaItemCount(t *testing.T) {
	planks := Planks{Wood: OakWood()}.FuelInfo()
	coal := item.Coal{}.FuelInfo()
	for _, test := range smelterCases {
		t.Run(test.name, func(t *testing.T) {
			sm := test.build()
			if got := sm.CookDuration(); got != test.cook {
				t.Errorf("cook duration = %v, want %v", got, test.cook)
			}
			if got := sm.ItemsPerFuel(planks); got != 1.5 {
				t.Errorf("items per plank = %v, want 1.5", got)
			}
			if got := sm.ItemsPerFuel(coal); got != 8 {
				t.Errorf("items per coal = %v, want 8", got)
			}
			if got := sm.ItemsPerFuel(longFuel{}.FuelInfo()); got != 95040 {
				t.Errorf("items per 11-day fuel = %v, want 95040", got)
			}

			w := world.Config{Synchronous: true}.New()
			defer w.Close()
			runWorld(w, func(tx *world.Tx) {
				pos := cube.Pos{0, 64, 0}
				tx.SetBlock(pos, sm, nil)
				inv := sm.Inventory(tx, pos)
				_ = inv.SetItem(SmelterInputSlot, test.input)
				_ = inv.SetItem(SmelterFuelSlot, item.NewStack(item.Coal{}, 1))
				tickSmelterBlock(tx, pos, 2000)
				if result, _ := inv.Item(SmelterResultSlot); result.Count() != 8 {
					t.Errorf("smelted %d items with one coal, want 8", result.Count())
				}
			})
		})
	}
}

func TestSmelter_CooledProgressStaysNonNegative(t *testing.T) {
	for _, test := range smelterCases {
		for _, savedCook := range []int16{-1, 0} {
			t.Run(fmt.Sprintf("%s saved %d", test.name, savedCook), func(t *testing.T) {
				sm := test.build()
				w := world.Config{Synchronous: true}.New()
				defer w.Close()
				runWorld(w, func(tx *world.Tx) {
					pos := cube.Pos{0, 64, 0}
					viewer := &furnaceProgressViewer{}
					tx.SetBlock(pos, sm, nil)
					sm.AddViewer(viewer, tx, pos)
					inv := sm.Inventory(tx, pos)
					_ = inv.SetItem(SmelterInputSlot, test.input)
					_ = inv.SetItem(SmelterFuelSlot, item.NewStack(item.Stick{}, 1))
					tickSmelterBlock(tx, pos, 200)

					data := tx.Block(pos).(testSmelter).EncodeNBT()
					if cook := data["CookTime"].(int16); cook < 0 {
						t.Errorf("saved CookTime = %d, want non-negative", cook)
					}
					if savedCook < 0 {
						data["CookTime"] = savedCook
					}
					decoded := tx.Block(pos).(testSmelter).DecodeNBT(data).(testSmelter)
					if _, _, cook := decoded.Durations(); cook < 0 {
						t.Errorf("decoded cook progress = %v, want non-negative", cook)
					}
					tx.SetBlock(pos, decoded, nil)
					decoded.AddViewer(viewer, tx, pos)
					inv = decoded.Inventory(tx, pos)
					_ = inv.SetItem(SmelterFuelSlot, item.NewStack(item.Coal{}, 1))
					tickSmelterBlock(tx, pos, 2000)

					if viewer.minCook < 0 {
						t.Errorf("viewer saw cook progress %v, want non-negative", viewer.minCook)
					}
					if result, _ := inv.Item(SmelterResultSlot); result.Count() != 8 {
						t.Errorf("smelted %d items with a stick then a coal, want 8", result.Count())
					}
				})
			})
		}
	}
}

func TestSmelter_LongFuelAdvancesCooking(t *testing.T) {
	for _, test := range smelterCases {
		t.Run(test.name, func(t *testing.T) {
			sm := test.build()
			w := world.Config{Synchronous: true}.New()
			defer w.Close()
			runWorld(w, func(tx *world.Tx) {
				pos := cube.Pos{0, 64, 0}
				tx.SetBlock(pos, sm, nil)
				inv := sm.Inventory(tx, pos)
				_ = inv.SetItem(SmelterInputSlot, test.input)
				_ = inv.SetItem(SmelterFuelSlot, item.NewStack(longFuel{}, 1))
				tickSmelterBlock(tx, pos, 1)

				remaining, maximum, cook := sm.Durations()
				if want := test.cook * 95040; maximum != want || remaining != want-time.Millisecond*50 {
					t.Errorf("burn = %v of %v, want %v of %v", remaining, maximum, want-time.Millisecond*50, want)
				}
				if cook != time.Millisecond*50 {
					t.Errorf("cook progress = %v, want 50ms", cook)
				}
			})
		})
	}
}
