package entity

import "testing"

func TestFallDamage(t *testing.T) {
	for _, test := range []struct {
		distance  float64
		jumpBoost int
		want      float64
	}{
		{distance: 3, want: 0},
		{distance: 3.49, want: 0},
		{distance: 3.5, want: 1},
		{distance: 10, want: 7},
		{distance: 10.2, want: 8},
		{distance: 10, jumpBoost: 2, want: 5},
		{distance: 5, jumpBoost: 2, want: 0},
	} {
		if got := FallDamage(test.distance, test.jumpBoost); got != test.want {
			t.Errorf("FallDamage(%g, %d) = %g, want %g", test.distance, test.jumpBoost, got, test.want)
		}
	}
}
