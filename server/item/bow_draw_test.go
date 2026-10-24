package item

import (
	"math"
	"testing"
)

func TestBowDrawPower_ReleaseThresholdAndCharge(t *testing.T) {
	for _, test := range []struct {
		ticks int
		want  float64
	}{
		{-1, 0}, {0, 0}, {2, 0},
		{3, .1075}, {5, .1875}, {10, 5.0 / 12}, {19, .9341666666666667},
		{20, 1}, {40, 1},
	} {
		if got := BowDrawPower(test.ticks); math.Abs(got-test.want) > 1e-12 {
			t.Fatalf("power after %d ticks = %g, want %g", test.ticks, got, test.want)
		}
	}
}
