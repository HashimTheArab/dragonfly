package block

import (
	"math"
	"math/rand/v2"
	"testing"
)

// summedTicks is the plain float32 accumulation breakTicks skips through in bulk.
func summedTicks(rate float32) (int64, bool) {
	var total float32
	for n := int64(1); n <= 30_000_000; n++ {
		next := total + rate
		if next == total {
			return 0, false
		}
		if total = next; total >= breakThreshold {
			return n, true
		}
	}
	return 0, false
}

// TestBreakTicksSkipsExactly checks that skipping whole binades lands on the same tick the addition-by-
// addition sum does, including on rates whose ulp offset ties and rounds the first addition of a binade
// the other way.
func TestBreakTicksSkipsExactly(t *testing.T) {
	check := func(rate float32) {
		t.Helper()
		wantN, wantOK := summedTicks(rate)
		gotN, gotOK := breakTicks(rate)
		if gotOK != wantOK || (wantOK && gotN != wantN) {
			t.Fatalf("rate %v (%#x): got (%d, %v), want (%d, %v)", rate, math.Float32bits(rate), gotN, gotOK, wantN, wantOK)
		}
	}
	// Every progress a vanilla hardness, tool speed, Efficiency addend, harvest divisor and 5x penalty
	// can combine into.
	for _, hardness := range []float32{0.1, 0.2, 0.25, 0.4, 0.5, 0.6, 0.8, 1, 1.5, 2, 2.5, 3, 3.5, 4, 5, 6, 15, 17.5, 22.5, 30, 50} {
		for _, speed := range []float32{1, 1.5, 2, 4, 5, 6, 8, 9, 12, 15, 30} {
			for _, efficiency := range []float32{0, 2, 5, 10, 17, 26} {
				for _, divisor := range []float32{30, 100} {
					for _, penalty := range []float32{1, 5, 25} {
						check((speed + efficiency) / penalty / hardness / divisor)
					}
				}
			}
		}
	}
	r := rand.New(rand.NewPCG(7, 11))
	for range 4000 {
		check(math.Float32frombits(r.Uint32N(0x3f800000-0x38000000) + 0x38000000))
		check(1 / float32(r.IntN(50000)+1))
	}
	// A rate under half an ulp of the running total leaves it stuck short of 1 forever.
	for range 200 {
		check(math.Float32frombits(r.Uint32N(0x33000000) + 1))
	}
}
