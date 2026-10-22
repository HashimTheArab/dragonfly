package player

import "testing"

// Rocket A boosts for 30 ticks and a shorter B is rejected. Showing A to a late viewer changes nothing, so once A has
// 15 ticks left, C lasting 20 still replaces it.
func TestGlideBoostReplacesOnlyWhenItLastsAtLeastAsLong(t *testing.T) {
	t.Parallel()
	left := int64(0)
	launch := func(ticks int64) bool {
		if !glideBoostReplaces(left, ticks) {
			return false
		}
		left = ticks
		return true
	}
	if !launch(30) || launch(20) {
		t.Fatal("A was rejected or the shorter B replaced it")
	}
	for range 15 {
		left--
	}
	if !launch(20) || left != 20 {
		t.Fatalf("C lasting 20 did not replace A with 15 left; left = %d", left)
	}
	for _, c := range []struct {
		current, ticks int64
		want           bool
	}{
		{0, 1, true},
		{20, 20, true},
		{-1, 100, false},
		{-1, -1, true},
		{20, -1, true},
	} {
		if got := glideBoostReplaces(c.current, c.ticks); got != c.want {
			t.Fatalf("glideBoostReplaces(%d, %d) = %v, want %v", c.current, c.ticks, got, c.want)
		}
	}
}
