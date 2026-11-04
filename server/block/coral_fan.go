package block

import (
	"math/rand/v2"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// CoralFan is a non-solid floor block that comes in 5 variants and dies when it is no longer touching water.
type CoralFan struct {
	empty
	transparent
	bassDrum
	sourceWaterDisplacer

	// Type is the type of coral of the block.
	Type CoralType
	// Dead is whether the coral fan is dead.
	Dead bool
	// AlongZ is whether the fan is laid out along the Z axis instead of the X axis.
	AlongZ bool
}

// UseOnBlock ...
func (c CoralFan) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, face, used := firstReplaceable(tx, pos, face, c)
	if !used || face == cube.FaceDown {
		return false
	}
	if !coralPlaceableIn(pos, tx) {
		return false
	}
	if !coralFanWaterlogged(pos, tx) {
		c.Dead = true
	}
	if face != cube.FaceUp {
		supportPos := pos.Side(face.Opposite())
		if !tx.Block(supportPos).Model().FaceSolid(supportPos, face, tx) {
			return false
		}
		place(tx, pos, CoralWallFan{Type: c.Type, Dead: c.Dead, Facing: face.Direction()}, user, ctx)
		return placed(ctx)
	}
	below := pos.Side(cube.FaceDown)
	if !tx.Block(below).Model().FaceSolid(below, cube.FaceUp, tx) {
		return false
	}

	c.AlongZ = user.Rotation().Direction().Face().Axis() == cube.X
	place(tx, pos, c, user, ctx)
	return placed(ctx)
}

// HasLiquidDrops ...
func (c CoralFan) HasLiquidDrops() bool {
	return false
}

// SideClosed ...
func (c CoralFan) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// NeighbourUpdateTick ...
func (c CoralFan) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	below := pos.Side(cube.FaceDown)
	if !tx.Block(below).Model().FaceSolid(below, cube.FaceUp, tx) {
		breakBlock(c, pos, tx)
		return
	} else if c.Dead {
		return
	}
	tx.ScheduleBlockUpdate(pos, c, time.Second*5/2)
}

// ScheduledTick ...
func (c CoralFan) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if !coralFanWaterlogged(pos, tx) {
		c.Dead = true
		tx.SetBlock(pos, c, nil)
	}
}

// BreakInfo ...
func (c CoralFan) BreakInfo() BreakInfo {
	return newBreakInfo(0, alwaysHarvestable, nothingEffective, silkTouchOnlyDrop(CoralFan{Type: c.Type, Dead: c.Dead}))
}

// EncodeBlock ...
func (c CoralFan) EncodeBlock() (name string, properties map[string]any) {
	properties = map[string]any{"coral_fan_direction": int32(boolByte(c.AlongZ))}
	if c.Dead {
		return "minecraft:dead_" + c.Type.String() + "_coral_fan", properties
	}
	return "minecraft:" + c.Type.String() + "_coral_fan", properties
}

// EncodeItem ...
func (c CoralFan) EncodeItem() (name string, meta int16) {
	if c.Dead {
		return "minecraft:dead_" + c.Type.String() + "_coral_fan", 0
	}
	return "minecraft:" + c.Type.String() + "_coral_fan", 0
}

// allCoralFan returns a list of all coral fan variants.
func allCoralFan() (c []world.Block) {
	f := func(dead bool) {
		for _, t := range CoralTypes() {
			c = append(c, CoralFan{Type: t, Dead: dead})
			c = append(c, CoralFan{Type: t, Dead: dead, AlongZ: true})
		}
	}
	f(true)
	f(false)
	return
}

// coralFanWaterlogged reports whether a floor or wall fan retains water in its own block layer.
func coralFanWaterlogged(pos cube.Pos, tx *world.Tx) bool {
	liquid, ok := tx.Liquid(pos)
	water, waterlogged := liquid.(Water)
	return ok && waterlogged && !water.Falling && water.Depth == 8
}
