package block

import (
	"time"

	"github.com/df-mc/dragonfly/server/item"
)

const (
	furnaceCookDuration     = time.Second * 10
	fastSmelterCookDuration = time.Second * 5
)

// CookDuration returns how long a furnace takes to smelt one item.
func (Furnace) CookDuration() time.Duration { return furnaceCookDuration }

// CookDuration returns how long a blast furnace takes to smelt one item.
func (BlastFurnace) CookDuration() time.Duration { return fastSmelterCookDuration }

// CookDuration returns how long a smoker takes to smelt one item.
func (Smoker) CookDuration() time.Duration { return fastSmelterCookDuration }

// ItemsPerFuel returns how many items one unit of the fuel smelts in a furnace.
func (f Furnace) ItemsPerFuel(fuel item.FuelInfo) float64 {
	return itemsPerFuel(fuel, f.CookDuration())
}

// ItemsPerFuel returns how many items one unit of the fuel smelts in a blast furnace.
func (b BlastFurnace) ItemsPerFuel(fuel item.FuelInfo) float64 {
	return itemsPerFuel(fuel, b.CookDuration())
}

// ItemsPerFuel returns how many items one unit of the fuel smelts in a smoker.
func (s Smoker) ItemsPerFuel(fuel item.FuelInfo) float64 {
	return itemsPerFuel(fuel, s.CookDuration())
}

func itemsPerFuel(fuel item.FuelInfo, cook time.Duration) float64 {
	return float64(fuelBurnDuration(fuel, cook)) / float64(cook)
}

// fuelBurnDuration scales a fuel's furnace burn time to a smelter that cooks in
// the given duration, so faster smelters burn their fuel proportionally faster.
func fuelBurnDuration(fuel item.FuelInfo, cook time.Duration) time.Duration {
	return fuel.Duration / time.Millisecond * cook / furnaceCookDuration * time.Millisecond
}
