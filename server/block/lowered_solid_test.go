package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

var loweredSolids = []struct {
	name  string
	block world.Block
}{
	{"mud", Mud{}},
	{"soul_sand", SoulSand{}},
}

func TestLoweredSolid_BedSafeSpawnSkipsBlock(t *testing.T) {
	for _, tc := range loweredSolids {
		t.Run(tc.name, func(t *testing.T) {
			w := world.Config{Synchronous: true}.New()
			defer w.Close()

			head := cube.Pos{0, 64, 0}
			foot := head.Side(cube.FaceSouth)
			blocked := foot.Add(cube.Pos{-1, 0, 0})
			var spawn cube.Pos
			var ok bool
			runWorld(w, func(tx *world.Tx) {
				for x := -3; x <= 3; x++ {
					for z := -3; z <= 3; z++ {
						tx.SetBlock(cube.Pos{x, 63, z}, Stone{}, nil)
					}
				}
				tx.SetBlock(head, Bed{Facing: cube.North, Head: true}, nil)
				tx.SetBlock(foot, Bed{Facing: cube.North}, nil)
				tx.SetBlock(blocked, tc.block, nil)
				spawn, ok = (Bed{Facing: cube.North, Head: true}).SafeSpawn(head, tx)
			})
			if !ok {
				t.Fatal("no safe spawn found")
			}
			if spawn == blocked {
				t.Fatalf("safe spawn %v is inside %s", spawn, tc.name)
			}
		})
	}
}

func TestLoweredSolid_SupportsVines(t *testing.T) {
	for _, tc := range loweredSolids {
		t.Run(tc.name, func(t *testing.T) {
			w := world.Config{Synchronous: true}.New()
			defer w.Close()

			support := cube.Pos{0, 64, 0}
			placedPos, kept := support.Side(cube.FaceEast), support.Side(cube.FaceNorth)
			var used bool
			var placed, updated world.Block
			runWorld(w, func(tx *world.Tx) {
				tx.SetBlock(support, tc.block, nil)
				user := &coralFanTestUser{tx: tx, held: item.NewStack(Vines{}, 1)}
				used = (Vines{}).UseOnBlock(support, cube.FaceEast, mgl64.Vec3{}, tx, user, &item.UseContext{})
				placed = tx.Block(placedPos)

				tx.SetBlock(kept, Vines{SouthDirection: true}, nil)
				tx.Block(kept).(Vines).NeighbourUpdateTick(kept, support, tx)
				updated = tx.Block(kept)
			})
			if v, ok := placed.(Vines); !used || !ok || !v.WestDirection {
				t.Errorf("vine placed on %s: used = %v, block = %#v", tc.name, used, placed)
			}
			if v, ok := updated.(Vines); !ok || !v.SouthDirection {
				t.Errorf("vine backed by %s after neighbour update = %#v", tc.name, updated)
			}
		})
	}
}
