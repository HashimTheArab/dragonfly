package block_test

import (
	"math"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type uniformBlockSource struct {
	block world.Block
}

type panicBlockSource struct{}

// Block returns the same block at every position.
func (s uniformBlockSource) Block(cube.Pos) world.Block {
	return s.block
}

// Block panics if exposure sampling attempts to resolve a block.
func (panicBlockSource) Block(cube.Pos) world.Block {
	panic("unexpected block lookup")
}

func TestExplosionExposureNearZeroRay(t *testing.T) {
	c := block.ExplosionConfig{}
	src := uniformBlockSource{block: block.Air{}}
	box := cube.Box(1e-11, 0, 0, 1e-11, 0, 0)

	if got := c.Exposure(src, mgl64.Vec3{}, box); got != 1 {
		t.Fatalf("expected full exposure for near-zero rays, got %v", got)
	}
}

func TestExplosionExposureOriginInsideClosedBox(t *testing.T) {
	c := block.ExplosionConfig{}
	box := cube.Box(0, 0, 0, 1, 1, 1)
	origin := mgl64.Vec3{0, 0.5, 0.5}

	if got := c.Exposure(panicBlockSource{}, origin, box); got != 1 {
		t.Fatalf("expected full exposure for an origin on the box boundary, got %v", got)
	}
}

func TestExplosionExposureRayContainedInsideBlock(t *testing.T) {
	c := block.ExplosionConfig{}
	src := uniformBlockSource{block: block.Stone{}}
	box := cube.Box(0.75, 0.75, 0.75, 0.75, 0.75, 0.75)
	origin := mgl64.Vec3{0.25, 0.25, 0.25}

	if got := c.Exposure(src, origin, box); got != 1 {
		t.Fatalf("expected full exposure for rays contained inside a block, got %v", got)
	}
}

func TestExplosionExposureNonFiniteBox(t *testing.T) {
	c := block.ExplosionConfig{}
	src := uniformBlockSource{block: block.Air{}}
	box := cube.Box(0, 0, 0, math.Inf(1), 1, 1)
	result := make(chan float64, 1)

	go func() {
		result <- c.Exposure(src, mgl64.Vec3{2, 2, 2}, box)
	}()

	select {
	case got := <-result:
		if got != 0 {
			t.Fatalf("expected zero exposure for a non-finite box, got %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("exposure did not return for a non-finite box")
	}
}

func TestExplosionExposureNonFiniteOrigin(t *testing.T) {
	c := block.ExplosionConfig{}
	src := uniformBlockSource{block: block.Air{}}
	box := cube.Box(0, 0, 0, 1, 1, 1)
	result := make(chan float64, 1)

	go func() {
		result <- c.Exposure(src, mgl64.Vec3{math.NaN(), 2, 2}, box)
	}()

	select {
	case got := <-result:
		if got != 0 {
			t.Fatalf("expected zero exposure for a non-finite origin, got %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("exposure did not return for a non-finite origin")
	}
}
