package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/enchantment"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// coralFanTestUser is the smallest item user needed to exercise block placement.
type coralFanTestUser struct {
	tx       *world.Tx
	rotation cube.Rotation
	held     item.Stack
}

// Close closes the test user.
func (*coralFanTestUser) Close() error { return nil }

// H returns no handle because the placement tests do not add the user to a world.
func (*coralFanTestUser) H() *world.EntityHandle { return nil }

// Position returns the origin for the test user.
func (*coralFanTestUser) Position() mgl64.Vec3 { return mgl64.Vec3{} }

// Rotation returns the configured test rotation.
func (u *coralFanTestUser) Rotation() cube.Rotation { return u.rotation }

// HeldItems returns the configured main-hand stack.
func (u *coralFanTestUser) HeldItems() (item.Stack, item.Stack) { return u.held, item.Stack{} }

// SetHeldItems replaces the configured main-hand stack.
func (u *coralFanTestUser) SetHeldItems(mainHand, _ item.Stack) { u.held = mainHand }

// UsingItem reports that the test user is not using an item continuously.
func (*coralFanTestUser) UsingItem() bool { return false }

// ReleaseItem ends a continued item use for the test user.
func (*coralFanTestUser) ReleaseItem() {}

// UseItem begins a continued item use for the test user.
func (*coralFanTestUser) UseItem() {}

// PlaceBlock places b through the active test transaction.
func (u *coralFanTestUser) PlaceBlock(pos cube.Pos, b world.Block, ctx *item.UseContext) {
	u.tx.SetBlock(pos, b, nil)
	ctx.SubtractFromCount(1)
}

func TestCoralFan_ItemAndSilkTouchDropUseRegisteredState(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	assertRegistered := func(name string, b world.Block) {
		t.Helper()
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("%s runtime ID lookup panicked: %v", name, recovered)
			}
		}()
		_ = world.BlockRuntimeID(b)
	}

	assertRegistered("item", CoralFan{Type: TubeCoral()})
	floor, ok := world.BlockByName("minecraft:tube_coral_fan", map[string]any{"coral_fan_direction": int32(0)})
	if !ok {
		t.Fatal("registered tube coral fan floor state not found")
	}
	drops := floor.(CoralFan).BreakInfo().Drops(item.ToolNone{}, []item.Enchantment{item.NewEnchantment(enchantment.SilkTouch, 1)})
	if len(drops) != 1 {
		t.Fatalf("silk touch drops = %v, want one coral fan", drops)
	}
	assertRegistered("silk touch drop", drops[0].Item().(world.Block))
}

func TestCoralFan_WallPlacementUsesWallState(t *testing.T) {
	w := world.Config{Synchronous: true}.New()
	defer w.Close()

	support := cube.Pos{0, 64, 0}
	target := support.Side(cube.FaceEast)
	var used bool
	var placed world.Block
	runWorld(w, func(tx *world.Tx) {
		tx.SetBlock(support, Stone{}, nil)
		tx.SetLiquid(target, Water{Depth: 8})
		user := &coralFanTestUser{tx: tx, held: item.NewStack(CoralFan{Type: TubeCoral()}, 1)}
		used = (CoralFan{Type: TubeCoral()}).UseOnBlock(support, cube.FaceEast, mgl64.Vec3{}, tx, user, &item.UseContext{})
		placed = tx.Block(target)
	})
	if !used {
		t.Fatal("coral fan side placement was rejected")
	}
	name, properties := placed.EncodeBlock()
	if name != "minecraft:tube_coral_wall_fan" {
		t.Fatalf("placed block name = %q, want wall coral fan", name)
	}
	if got := properties["coral_direction"]; got != int32(1) {
		t.Fatalf("coral_direction = %v, want east mapping 1", got)
	}
}

func TestCoralFan_WaterloggedFanSurvivesScheduledTick(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	floor, ok := world.BlockByName("minecraft:tube_coral_fan", map[string]any{"coral_fan_direction": int32(0)})
	if !ok {
		t.Fatal("registered tube coral fan floor state not found")
	}

	w := world.Config{Synchronous: true}.New()
	defer w.Close()
	pos := cube.Pos{0, 64, 0}
	var after CoralFan
	runWorld(w, func(tx *world.Tx) {
		tx.SetBlock(pos.Side(cube.FaceDown), Stone{}, nil)
		tx.SetBlock(pos, floor, nil)
		tx.SetLiquid(pos, Water{Depth: 8})
		floor.(CoralFan).ScheduledTick(pos, tx, nil)
		after = tx.Block(pos).(CoralFan)
	})
	if after.Dead {
		t.Fatal("waterlogged coral fan died despite retaining source water")
	}
}

func TestCoralWallFan_NeighbourUpdateSchedulesDeathWithoutWater(t *testing.T) {
	w := world.Config{Synchronous: true}.New()
	defer w.Close()
	pos := cube.Pos{0, 64, 0}
	runWorld(w, func(tx *world.Tx) {
		tx.SetBlock(pos.Side(cube.FaceWest), Stone{}, nil)
		fan := CoralWallFan{Type: TubeCoral(), Facing: cube.East}
		tx.SetBlock(pos, fan, nil)
		fan.NeighbourUpdateTick(pos, pos.Side(cube.FaceWest), tx)
	})
	for range 51 {
		w.AdvanceTick()
	}
	var after CoralWallFan
	runWorld(w, func(tx *world.Tx) {
		after = tx.Block(pos).(CoralWallFan)
	})
	if !after.Dead {
		t.Fatal("live wall coral fan did not die after losing water")
	}
}
