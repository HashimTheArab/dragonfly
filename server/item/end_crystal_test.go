package item_test

import (
	"fmt"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func TestEndCrystalPlacementUsesJavaEntityCollisionColumn(t *testing.T) {
	w := newEndCrystalTestWorld()
	defer w.Close()

	var testErr error
	<-w.Exec(func(tx *world.Tx) {
		pos := cube.Pos{0, 0, 0}
		tx.SetBlock(pos, block.Obsidian{}, nil)
		tx.AddEntity(world.EntitySpawnOpts{Position: mgl64.Vec3{1.31, 1, 0.5}}.New(testCrystalBlockingEntityType{}, testCrystalBlockingEntityBehaviour{}))

		var ctx item.UseContext
		if !(item.EndCrystal{}).UseOnBlock(pos, cube.FaceUp, mgl64.Vec3{0.5, 1, 0.5}, tx, nil, &ctx) {
			testErr = fmt.Errorf("end crystal placement was blocked by entity outside Java placement AABB")
			return
		}
		if ctx.CountSub != 1 {
			testErr = fmt.Errorf("CountSub = %d, want 1", ctx.CountSub)
		}
	})
	if testErr != nil {
		t.Fatal(testErr)
	}
}

func TestEndCrystalPlacementBlocksEntitiesInsideJavaCollisionColumn(t *testing.T) {
	w := newEndCrystalTestWorld()
	defer w.Close()

	var placed bool
	<-w.Exec(func(tx *world.Tx) {
		pos := cube.Pos{0, 0, 0}
		tx.SetBlock(pos, block.Obsidian{}, nil)
		tx.AddEntity(world.EntitySpawnOpts{Position: mgl64.Vec3{0.5, 1, 0.5}}.New(testCrystalBlockingEntityType{}, testCrystalBlockingEntityBehaviour{}))

		var ctx item.UseContext
		placed = (item.EndCrystal{}).UseOnBlock(pos, cube.FaceUp, mgl64.Vec3{0.5, 1, 0.5}, tx, nil, &ctx)
	})

	if placed {
		t.Fatal("end crystal placement succeeded despite entity inside Java placement AABB")
	}
}

func TestEndCrystalPlacementRequiresOnlyBlockAboveClear(t *testing.T) {
	w := newEndCrystalTestWorld()
	defer w.Close()

	var placed bool
	<-w.Exec(func(tx *world.Tx) {
		pos := cube.Pos{0, 0, 0}
		tx.SetBlock(pos, block.Obsidian{}, nil)
		tx.SetBlock(pos.Side(cube.FaceUp).Side(cube.FaceUp), block.Stone{}, nil)

		var ctx item.UseContext
		placed = (item.EndCrystal{}).UseOnBlock(pos, cube.FaceUp, mgl64.Vec3{0.5, 1, 0.5}, tx, nil, &ctx)
	})

	if !placed {
		t.Fatal("end crystal placement failed even though Java only requires the block directly above to be clear")
	}
}

func newEndCrystalTestWorld() *world.World {
	return world.Config{Entities: world.EntityRegistryConfig{EndCrystal: entity.NewEndCrystal}.New([]world.EntityType{
		entity.EndCrystalType,
		testCrystalBlockingEntityType{},
	})}.New()
}

type testCrystalBlockingEntityType struct{}

func (testCrystalBlockingEntityType) Open(tx *world.Tx, h *world.EntityHandle, data *world.EntityData) world.Entity {
	return entity.Open(tx, h, data)
}

func (testCrystalBlockingEntityType) EncodeEntity() string {
	return "dragonfly:test_crystal_blocking_entity"
}

func (testCrystalBlockingEntityType) BBox(world.Entity) cube.BBox {
	return cube.Box(-0.3, 0, -0.3, 0.3, 1.8, 0.3)
}

func (testCrystalBlockingEntityType) DecodeNBT(map[string]any, *world.EntityData) {}
func (testCrystalBlockingEntityType) EncodeNBT(*world.EntityData) map[string]any  { return nil }

type testCrystalBlockingEntityBehaviour struct{}

func (b testCrystalBlockingEntityBehaviour) Apply(data *world.EntityData) {
	data.Data = b
}

func (testCrystalBlockingEntityBehaviour) Tick(*entity.Ent, *world.Tx) *entity.Movement {
	return nil
}
