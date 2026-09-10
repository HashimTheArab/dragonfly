package trace_test

import (
	"math"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type visibilityWorld map[cube.Pos]world.Block

func (w visibilityWorld) Block(pos cube.Pos) world.Block {
	if pos.Y() < -64 || pos.Y() > 319 {
		panic("out-of-range block read")
	}
	if b, ok := w[pos]; ok {
		return b
	}
	return block.Air{}
}
func TestBlockVisibility(t *testing.T) {
	start, end := mgl64.Vec3{0.5, 0.5, 0.5}, mgl64.Vec3{3.5, 0.5, 0.5}
	wall := cube.Pos{2, 0, 0}
	for _, tt := range []struct {
		name       string
		start, end mgl64.Vec3
		blocks     visibilityWorld
		ignored    []cube.Pos
		want       trace.Visibility
	}{
		{"clear", start, end, nil, nil, trace.VisibilityClear},
		{"blocked", start, end, visibilityWorld{wall: block.Stone{}}, nil, trace.VisibilityBlocked},
		{"ignored endpoint", start, wall.Vec3Centre(), visibilityWorld{wall: block.Stone{}}, []cube.Pos{wall}, trace.VisibilityClear},
		{"other obstruction", start, end, visibilityWorld{wall: block.Stone{}}, []cube.Pos{{3, 0, 0}}, trace.VisibilityBlocked},
		{"zero length", start, start, nil, nil, trace.VisibilityClear},
		{"near zero", start, start.Add(mgl64.Vec3{1e-8, 0, 0}), nil, nil, trace.VisibilityClear},
		{"below bounds", mgl64.Vec3{0, -65, 0}, end, nil, nil, trace.VisibilityInvalid},
		{"ignored outside bounds", start, mgl64.Vec3{0, 320, 0}, nil, []cube.Pos{{0, 320, 0}}, trace.VisibilityInvalid},
		{"nonfinite", start, mgl64.Vec3{math.NaN(), 0, 0}, nil, nil, trace.VisibilityInvalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := trace.BlockVisibility(tt.blocks, cube.Range{-64, 319}, tt.start, tt.end, tt.ignored...); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
