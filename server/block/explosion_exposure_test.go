package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// blockSource resolves a fixed set of positions, standing in for a world.
type blockSource map[cube.Pos]world.Block

func (s blockSource) Block(pos cube.Pos) world.Block {
	if b, ok := s[pos]; ok {
		return b
	}
	return Air{}
}

func TestExposure(t *testing.T) {
	box := cube.Box(0, 0, 0, 1, 1, 1).Translate(mgl64.Vec3{10, 0, 0})
	origin := mgl64.Vec3{0, 0.5, 0.5}

	if got := (ExplosionConfig{}).Exposure(blockSource{}, origin, box); got != 1 {
		t.Fatalf("open line of sight = %v, want 1", got)
	}

	// A wall of stone across the path stops every ray.
	walled := blockSource{}
	for y := -1; y <= 2; y++ {
		for z := -1; z <= 2; z++ {
			walled[cube.Pos{5, y, z}] = Stone{}
		}
	}
	if got := (ExplosionConfig{}).Exposure(walled, origin, box); got != 0 {
		t.Fatalf("fully walled off = %v, want 0", got)
	}

	// A degenerate box has nothing to sample.
	degenerate := cube.Box(0, 0, 0, 0, 0, 0)
	if got := (ExplosionConfig{}).Exposure(blockSource{}, mgl64.Vec3{}, degenerate); got != 1 {
		t.Fatalf("point box at the origin = %v, want 1 (nothing can occlude it)", got)
	}
}
