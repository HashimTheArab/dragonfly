package entity

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type pearlPredictionBlocks struct{}

func (pearlPredictionBlocks) Block(pos cube.Pos) world.Block {
	if pos == (cube.Pos{0, 0, 3}) {
		return block.Stone{}
	}
	return block.Air{}
}

func TestPredictEnderPearlLanding_BlockAndUnknownTerrain(t *testing.T) {
	start, velocity := mgl64.Vec3{0.5, 0.5, 0.5}, mgl64.Vec3{0, 0, 1}
	hit, ok := PredictEnderPearlLanding(pearlPredictionBlocks{}, func(cube.Pos) bool { return true }, start, velocity, 0, 0, 8)
	if !ok || hit != (mgl64.Vec3{0.5, 0.5, 3}) {
		t.Fatalf("landing = %v, %t", hit, ok)
	}
	_, ok = PredictEnderPearlLanding(pearlPredictionBlocks{}, func(pos cube.Pos) bool { return pos.Z() < 2 }, start, velocity, 0, 0, 8)
	if ok {
		t.Fatal("unknown terrain produced a landing")
	}
}
