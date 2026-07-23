package entity_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func BenchmarkFallingBlockTick(b *testing.B) {
	blocks := []struct {
		name  string
		block world.Block
	}{
		{name: "Sand", block: block.Sand{}},
		{name: "Gravel", block: block.Gravel{}},
	}
	for _, falling := range blocks {
		for _, count := range []int{100, 1000} {
			b.Run(falling.name+"/"+strconv.Itoa(count)+"/Airborne", func(b *testing.B) {
				benchmarkFallingBlockTick(b, count, falling.block, false)
			})
			b.Run(falling.name+"/"+strconv.Itoa(count)+"/Landing", func(b *testing.B) {
				benchmarkFallingBlockTick(b, count, falling.block, true)
			})
		}
	}
}

func benchmarkFallingBlockTick(b *testing.B, count int, falling world.Block, landing bool) {
	b.ReportAllocs()
	b.ResetTimer()
	b.StopTimer()
	for range b.N {
		w := world.Config{Dim: world.End, Synchronous: true, Entities: entity.DefaultRegistry}.New()
		side := fallingBlockGridSide(count)
		task := w.Do(func(tx *world.Tx) {
			if landing {
				setupFallingBlockFloor(tx, side)
			}
			y := 128.0
			if landing {
				y = 64
			}
			for i := range count {
				x, z := i%side, i/side
				tx.AddEntity(entity.NewFallingBlock(world.EntitySpawnOpts{
					Position: mgl64.Vec3{float64(x) + 0.5, y, float64(z) + 0.5},
				}, falling))
			}
		})
		if err := task.Wait(context.Background()); err != nil {
			b.Fatalf("set up falling block benchmark: %v", err)
		}

		b.StartTimer()
		w.AdvanceTick()
		b.StopTimer()

		validateFallingBlockTick(b, w, count, side, landing)
		if err := w.Close(); err != nil {
			b.Fatalf("close falling block benchmark world: %v", err)
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*count), "ns/entity-tick")
}

func fallingBlockGridSide(count int) int {
	side := 1
	for side*side < count {
		side++
	}
	return side
}

func setupFallingBlockFloor(tx *world.Tx, side int) {
	opts := &world.SetOpts{
		DisableBlockUpdates:       true,
		DisableLiquidDisplacement: true,
		DisableRedstoneUpdates:    true,
	}
	for x := -1; x <= side; x++ {
		for z := -1; z <= side; z++ {
			tx.SetBlock(cube.Pos{x, 63, z}, block.Stone{}, opts)
		}
	}
}

func validateFallingBlockTick(b *testing.B, w *world.World, count, side int, landing bool) {
	b.Helper()
	got, err := world.Call(context.Background(), w, func(tx *world.Tx) (int, error) {
		if !landing {
			entities := 0
			for range tx.Entities() {
				entities++
			}
			return entities, nil
		}
		blocks := 0
		for i := range count {
			x, z := i%side, i/side
			if _, air := tx.Block(cube.Pos{x, 64, z}).(block.Air); !air {
				blocks++
			}
		}
		return blocks, nil
	})
	if err != nil {
		b.Fatalf("validate falling block benchmark: %v", err)
	}
	if got != count {
		b.Fatalf("falling blocks after tick = %d, want %d", got, count)
	}
}
