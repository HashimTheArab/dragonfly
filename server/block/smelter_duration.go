package block

import (
	"math"
	"math/bits"
	"time"

	"github.com/df-mc/dragonfly/server/item"
)

const (
	furnaceCookDuration     = time.Second * 10
	fastSmelterCookDuration = time.Second * 5
	smelterTick             = time.Millisecond * 50
	smelterCooldown         = smelterTick * 2
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
	return float64(fuelBurnDuration(fuel.Duration, cook)) / float64(cook)
}

// fuelBurnDuration scales a fuel's furnace burn time to a smelter that cooks in
// the given duration, so faster smelters burn their fuel proportionally faster.
// The result is whole ticks, and zero when the fuel cannot burn for one.
func fuelBurnDuration(fuel, cook time.Duration) time.Duration {
	if fuel <= 0 || cook <= 0 {
		return 0
	}
	hi, lo := bits.Mul64(uint64(fuel), uint64(cook))
	if hi >= uint64(furnaceCookDuration) {
		return time.Duration(math.MaxInt64).Truncate(smelterTick)
	}
	burn, _ := bits.Div64(hi, lo, uint64(furnaceCookDuration))
	return time.Duration(min(burn, math.MaxInt64)).Truncate(smelterTick)
}
