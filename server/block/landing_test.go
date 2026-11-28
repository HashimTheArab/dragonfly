package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type landingSource map[cube.Pos]world.Block

func (s landingSource) Block(pos cube.Pos) world.Block {
	if b, ok := s[pos]; ok {
		return b
	}
	return Air{}
}

// TestLandedOn finds the lander under any part of the box, not only under its centre.
func TestLandedOn(t *testing.T) {
	src := landingSource{{0, 63, 0}: Stone{}, {1, 63, 0}: HayBale{}}
	for _, test := range []struct {
		name    string
		centreX float64
		want    cube.Pos
		found   bool
	}{
		{name: "centre over stone, edge over hay", centreX: 0.8, want: cube.Pos{1, 63, 0}, found: true},
		{name: "on stone alone", centreX: 0.5},
		{name: "on the boundary over stone", centreX: 0.7},
	} {
		t.Run(test.name, func(t *testing.T) {
			box := cube.Box(-0.3, 0, -0.3, 0.3, 1.8, 0.3).Translate(mgl64.Vec3{test.centreX, 64, 0.5})
			pos, lander, ok := LandedOn(src, box)
			if ok != test.found || ok && (pos != test.want || lander == nil) {
				t.Fatalf("LandedOn = %v, %T, %v, want %v, %v", pos, lander, ok, test.want, test.found)
			}
		})
	}
}
