package block

import (
	"math/rand/v2"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// CoralFan is a non-solid block that comes in 5 variants and dies when it is no longer next to water.
type CoralFan struct {
	empty
	transparent
	bassDrum
	sourceWaterDisplacer

	// Type is the type of coral of the block.
	Type CoralType
	// Dead is whether the coral fan is dead.
	Dead bool
	// Axis is the horizontal axis the fan is laid out along. Only X and Z are valid.
	Axis cube.Axis
}

// UseOnBlock ...
func (c CoralFan) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, c)
	if !used {
		return false
	}
	below := pos.Side(cube.FaceDown)
	if !tx.Block(below).Model().FaceSolid(below, cube.FaceUp, tx) {
		return false
	}
	if liquid, ok := tx.Liquid(pos); ok {
		if water, ok := liquid.(Water); ok && water.Depth != 8 {
			return false
		}
	}

	c.Axis = cube.X
	if user.Rotation().Direction().Face().Axis() == cube.X {
		c.Axis = cube.Z
	}
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
	adjacentWater := false
	pos.Neighbours(func(neighbour cube.Pos) {
		if liquid, ok := tx.Liquid(neighbour); ok {
			if _, ok := liquid.(Water); ok {
				adjacentWater = true
			}
		}
	}, tx.Range())
	if !adjacentWater {
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
	properties = map[string]any{"coral_fan_direction": encodeCoralFanAxis(c.Axis)}
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

// encodeCoralFanAxis encodes the axis a coral fan is laid out along.
func encodeCoralFanAxis(axis cube.Axis) int32 {
	if axis == cube.X {
		return 0
	}
	return 1
}

// allCoralFan returns a list of all coral fan variants.
func allCoralFan() (c []world.Block) {
	f := func(dead bool) {
		for _, t := range CoralTypes() {
			c = append(c, CoralFan{Type: t, Dead: dead, Axis: cube.X})
			c = append(c, CoralFan{Type: t, Dead: dead, Axis: cube.Z})
		}
	}
	f(true)
	f(false)
	return
}
