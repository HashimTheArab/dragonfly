package block

import (
	"context"
	"encoding/binary"
	"hash/fnv"
	"math/rand/v2"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type fixedExplosionSource struct {
	pos mgl64.Vec3
}

func (s fixedExplosionSource) Position() mgl64.Vec3 { return s.pos }
func (fixedExplosionSource) Size() float64          { return 4 }

type captureExplosionHandler struct {
	world.NopHandler
	blocks []cube.Pos
}

func (h *captureExplosionHandler) HandleExplosion(
	ctx *world.Context,
	_ world.ExplosionSource,
	_ *[]world.Entity,
	blocks *[]cube.Pos,
	_ *float64,
	_ *bool,
) {
	h.blocks = append(h.blocks[:0], (*blocks)...)
	ctx.Cancel()
}

func TestExplosionAffectedBlockOrder(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*world.Tx)
		wantBlocks int
		wantHash   uint64
	}{
		{name: "air", wantBlocks: 1133, wantHash: 0x6becb2cf1543e82e},
		{name: "stone floor", setup: setupExplosionStoneFloor, wantBlocks: 670, wantHash: 0xbf4f3deb893ce184},
		{name: "water cube", setup: setupExplosionWaterCube, wantHash: 0xcbf29ce484222325},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := world.Config{Synchronous: true}.New()
			defer w.Close()

			handler := &captureExplosionHandler{}
			w.Handle(handler)
			task := w.Do(func(tx *world.Tx) {
				preloadExplosionTestChunks(tx)
				if test.setup != nil {
					test.setup(tx)
				}
				ExplosionConfig{RandSource: rand.NewPCG(1, 2)}.Explode(tx, fixedExplosionSource{
					pos: mgl64.Vec3{0.5, 64.5, 0.5},
				})
			})
			if err := task.Wait(context.Background()); err != nil {
				t.Fatalf("explode: %v", err)
			}

			gotHash := hashBlockPositions(handler.blocks)
			if len(handler.blocks) != test.wantBlocks || gotHash != test.wantHash {
				t.Fatalf("affected blocks = %d/%#x, want %d/%#x", len(handler.blocks), gotHash, test.wantBlocks, test.wantHash)
			}
		})
	}
}

func hashBlockPositions(positions []cube.Pos) uint64 {
	h := fnv.New64a()
	var buf [8]byte
	for _, pos := range positions {
		for _, coordinate := range pos {
			binary.LittleEndian.PutUint64(buf[:], uint64(int64(coordinate)))
			_, _ = h.Write(buf[:])
		}
	}
	return h.Sum64()
}

func preloadExplosionTestChunks(tx *world.Tx) {
	for x := -16; x <= 16; x += 16 {
		for z := -16; z <= 16; z += 16 {
			_ = tx.Block(cube.Pos{x, 64, z})
		}
	}
}

func setupExplosionStoneFloor(tx *world.Tx) {
	opts := &world.SetOpts{
		DisableBlockUpdates:       true,
		DisableLiquidDisplacement: true,
		DisableRedstoneUpdates:    true,
	}
	for x := -12; x <= 12; x++ {
		for z := -12; z <= 12; z++ {
			tx.SetBlock(cube.Pos{x, 63, z}, Stone{}, opts)
		}
	}
}

func setupExplosionWaterCube(tx *world.Tx) {
	water := Water{Depth: 8, Still: true}
	for x := -5; x <= 5; x++ {
		for y := 59; y <= 69; y++ {
			for z := -5; z <= 5; z++ {
				tx.SetLiquid(cube.Pos{x, y, z}, water)
			}
		}
	}
}
