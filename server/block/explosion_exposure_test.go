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

// liquidSource answers for liquids as well as blocks, which is what
// SuppressUnderwaterImpact needs.
type liquidSource struct {
	blockSource
	liquids map[cube.Pos]world.Liquid
}

func (s liquidSource) Liquid(pos cube.Pos) (world.Liquid, bool) {
	l, ok := s.liquids[pos]
	return l, ok
}

// SuppressUnderwaterImpact is only honoured by a source that can resolve liquids, so a
// plain BlockSource must not silently behave as though water were absent from the world.
func TestExposure_UnderwaterSuppressionNeedsALiquidSource(t *testing.T) {
	box := cube.Box(0, 0, 0, 1, 1, 1).Translate(mgl64.Vec3{10, 0, 0})
	origin := mgl64.Vec3{0, 0.5, 0.5}
	cfg := ExplosionConfig{SuppressUnderwaterImpact: true}

	water := map[cube.Pos]world.Liquid{}
	for y := -1; y <= 2; y++ {
		for z := -1; z <= 2; z++ {
			water[cube.Pos{5, y, z}] = Water{Depth: 8}
		}
	}
	if got := cfg.Exposure(liquidSource{blockSource{}, water}, origin, box); got != 0 {
		t.Fatalf("water wall with a LiquidSource = %v, want 0", got)
	}
	// The same wall through a block-only source: nothing to suppress against.
	if got := cfg.Exposure(blockSource{}, origin, box); got != 1 {
		t.Fatalf("block-only source = %v, want 1", got)
	}
}
