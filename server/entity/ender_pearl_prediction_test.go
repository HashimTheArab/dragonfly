package entity

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type pearlPredictionBlocks map[cube.Pos]world.Block

func (blocks pearlPredictionBlocks) Block(pos cube.Pos) world.Block {
	if b, ok := blocks[pos]; ok {
		return b
	}
	return block.Air{}
}

func TestPredictEnderPearlLanding_BlockAndUnknownTerrain(t *testing.T) {
	start, velocity := mgl64.Vec3{0.5, 0.5, 0.5}, mgl64.Vec3{0, 0, 1}
	blocks := pearlPredictionBlocks{{0, 0, 3}: block.Stone{}}
	hit, ok := PredictEnderPearlLanding(blocks, func(cube.Pos) bool { return true }, start, velocity, 0, 0, 8)
	if !ok || !hit.Hit || hit.Position != (mgl64.Vec3{0.5, 0.5, 3}) || hit.Block != (cube.Pos{0, 0, 3}) {
		t.Fatalf("landing = %v, %t", hit, ok)
	}
	partial, ok := PredictEnderPearlLanding(blocks, func(pos cube.Pos) bool { return pos.Z() < 2 }, start, velocity, 0, 0, 8)
	if !ok || partial.Hit || partial.Position != (mgl64.Vec3{0.5, 0.5, 1.5}) {
		t.Fatalf("unknown terrain = %v, %t", partial, ok)
	}
}

func TestPredictEnderPearlLanding_BDSMovementBeforeForces(t *testing.T) {
	for _, tt := range []struct {
		name          string
		velocity      mgl64.Vec3
		gravity, drag float64
		ticks         int
		want          mgl64.Vec3
	}{
		{"bds first tick", mgl64.Vec3{0, 0, 1.5}, 0.025, 0, 1, mgl64.Vec3{0.5, 10.5, 2}},
		{"bds third tick", mgl64.Vec3{0, 0, 1.5}, 0.025, 0, 3, mgl64.Vec3{0.5, 10.425, 5}},
		{"custom first tick", mgl64.Vec3{2, 1, 3}, 0.25, 0.5, 1, mgl64.Vec3{2.5, 11.5, 3.5}},
		{"custom third tick", mgl64.Vec3{2, 1, 3}, 0.25, 0.5, 3, mgl64.Vec3{4, 11.625, 5.75}},
		{"stationary apex", mgl64.Vec3{}, 0.25, 0.5, 2, mgl64.Vec3{0.5, 10.25, 0.5}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := PredictEnderPearlLanding(pearlPredictionBlocks{}, func(cube.Pos) bool { return true }, mgl64.Vec3{0.5, 10.5, 0.5}, tt.velocity, tt.gravity, tt.drag, tt.ticks)
			if !ok || got.Hit || got.Position.Sub(tt.want).Len() > 1e-10 {
				t.Fatalf("flight after %d ticks = %v, %t; want %v", tt.ticks, got, ok, tt.want)
			}
		})
	}
}

func TestPredictEnderPearlLanding_FirstTickBlockAtLaunchHeight(t *testing.T) {
	blocks := pearlPredictionBlocks{{0, 1, 1}: block.Stone{}}
	got, ok := PredictEnderPearlLanding(blocks, func(cube.Pos) bool { return true }, mgl64.Vec3{0.5, 1.01, 0.5}, mgl64.Vec3{0, 0, 1}, 0.025, 0, 1)
	want := mgl64.Vec3{0.5, 1.01, 1}
	if !ok || !got.Hit || got.Block != (cube.Pos{0, 1, 1}) || got.Position.Sub(want).Len() > 1e-10 {
		t.Fatalf("first tick collision = %v, %t; want block {0, 1, 1} at %v", got, ok, want)
	}
}
