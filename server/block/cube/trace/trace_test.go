package trace

import (
	"math"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

func TestTraverseBlocksReturnsForNonFiniteInput(t *testing.T) {
	tests := []struct {
		name  string
		start mgl64.Vec3
		end   mgl64.Vec3
	}{
		{name: "nan start", start: mgl64.Vec3{math.NaN(), 0, 0}, end: mgl64.Vec3{1, 0, 0}},
		{name: "nan end", start: mgl64.Vec3{}, end: mgl64.Vec3{1, math.NaN(), 0}},
		{name: "inf end", start: mgl64.Vec3{}, end: mgl64.Vec3{1, 0, math.Inf(1)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan bool)
			go func() {
				called := false
				TraverseBlocks(tt.start, tt.end, func(cube.Pos) bool {
					called = true
					return true
				})
				done <- called
			}()

			select {
			case called := <-done:
				if called {
					t.Fatal("TraverseBlocks called callback for non-finite input")
				}
			case <-time.After(100 * time.Millisecond):
				t.Fatal("TraverseBlocks did not return for non-finite input")
			}
		})
	}
}

func TestPerformReturnsForNonFiniteInput(t *testing.T) {
	if hit, ok := Perform(mgl64.Vec3{}, mgl64.Vec3{1, math.NaN(), 0}, nil, cube.Box(0, 0, 0, 1, 1, 1), nil); ok || hit != nil {
		t.Fatal("Perform returned a hit for non-finite input")
	}
}
