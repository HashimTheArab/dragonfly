package player

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func TestPlayer_InsideOfSolid(t *testing.T) {
	for _, tc := range []struct {
		name  string
		block world.Block
		want  bool
	}{
		{"stone", block.Stone{}, true},
		{"mud", block.Mud{}, true},
		{"soul_sand", block.SoulSand{}, true},
		{"glass", block.Glass{}, false},
		{"air", block.Air{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := world.Config{Synchronous: true}.New()
			t.Cleanup(func() { _ = w.Close() })

			var got bool
			if err := w.Do(func(tx *world.Tx) {
				tx.SetBlock(cube.Pos{0, 65, 0}, tc.block, nil)
				h := world.EntitySpawnOpts{}.New(Type, Config{Name: "test", Position: mgl64.Vec3{0.5, 64, 0.5}})
				got = tx.AddEntity(h).(*Player).insideOfSolid()
			}).Err(); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("insideOfSolid() = %v, want %v", got, tc.want)
			}
		})
	}
}
