package block_test

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type benchmarkExplosionSource struct {
	pos mgl64.Vec3
}

func (s benchmarkExplosionSource) Position() mgl64.Vec3 { return s.pos }
func (benchmarkExplosionSource) Size() float64          { return 4 }

type cancelExplosionHandler struct {
	world.NopHandler
	explosions     int
	affectedBlocks int
}

func (h *cancelExplosionHandler) HandleExplosion(
	ctx *world.Context,
	_ world.ExplosionSource,
	_ *[]world.Entity,
	blocks *[]cube.Pos,
	_ *float64,
	_ *bool,
) {
	h.explosions++
	h.affectedBlocks += len(*blocks)
	ctx.Cancel()
}

func BenchmarkExplosionSelection(b *testing.B) {
	scenarios := []struct {
		name  string
		setup func(*world.Tx)
	}{
		{name: "Air"},
		{name: "StoneFloor", setup: setupStoneFloor},
		{name: "WaterCube", setup: setupWaterCube},
	}
	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			benchmarkExplosionScenario(b, 1, scenario.setup)
		})
	}
}

func BenchmarkExplosionSelectionVolley100(b *testing.B) {
	benchmarkExplosionScenario(b, 100, setupStoneFloor)
}

func benchmarkExplosionScenario(b *testing.B, volleySize int, setup func(*world.Tx)) {
	w := world.Config{Synchronous: true}.New()
	defer w.Close()

	handler := &cancelExplosionHandler{}
	w.Handle(handler)
	source := benchmarkExplosionSource{pos: mgl64.Vec3{0.5, 64.5, 0.5}}

	_, err := world.Call(context.Background(), w, func(tx *world.Tx) (struct{}, error) {
		preloadExplosionChunks(tx)
		if setup != nil {
			setup(tx)
		}

		conf := block.ExplosionConfig{RandSource: rand.NewPCG(1, 2)}
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			for range volleySize {
				conf.Explode(tx, source)
			}
		}
		b.StopTimer()
		return struct{}{}, nil
	})
	if err != nil {
		b.Fatalf("run explosion benchmark: %v", err)
	}

	wantExplosions := b.N * volleySize
	if handler.explosions != wantExplosions {
		b.Fatalf("handled explosions = %d, want %d", handler.explosions, wantExplosions)
	}
	b.ReportMetric(float64(handler.affectedBlocks)/float64(wantExplosions), "affected-blocks/explosion")
	if volleySize > 1 {
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(wantExplosions), "ns/explosion")
	}
}

func preloadExplosionChunks(tx *world.Tx) {
	for x := -16; x <= 16; x += 16 {
		for z := -16; z <= 16; z += 16 {
			_ = tx.Block(cube.Pos{x, 64, z})
		}
	}
}

func setupStoneFloor(tx *world.Tx) {
	opts := &world.SetOpts{
		DisableBlockUpdates:       true,
		DisableLiquidDisplacement: true,
		DisableRedstoneUpdates:    true,
	}
	for x := -12; x <= 12; x++ {
		for z := -12; z <= 12; z++ {
			tx.SetBlock(cube.Pos{x, 63, z}, block.Stone{}, opts)
		}
	}
}

func setupWaterCube(tx *world.Tx) {
	water := block.Water{Depth: 8, Still: true}
	for x := -5; x <= 5; x++ {
		for y := 59; y <= 69; y++ {
			for z := -5; z <= 5; z++ {
				tx.SetLiquid(cube.Pos{x, y, z}, water)
			}
		}
	}
}
