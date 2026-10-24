package block

import (
	"math/rand/v2"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// CoralWallFan is a coral fan attached to a vertical block face.
type CoralWallFan struct {
	empty
	transparent
	bassDrum
	sourceWaterDisplacer

	// Type is the type of coral of the block.
	Type CoralType
	// Dead is whether the coral fan is dead.
	Dead bool
	// Facing is the direction the fan points away from its supporting block.
	Facing cube.Direction
}

// HasLiquidDrops always returns false.
func (CoralWallFan) HasLiquidDrops() bool { return false }

// SideClosed always returns false.
func (CoralWallFan) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool { return false }

// NeighbourUpdateTick breaks the fan when its supporting face is removed.
func (c CoralWallFan) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	supportPos := pos.Side(c.Facing.Face().Opposite())
	if !tx.Block(supportPos).Model().FaceSolid(supportPos, c.Facing.Face(), tx) {
		breakBlock(c, pos, tx)
		return
	}
	if !c.Dead {
		tx.ScheduleBlockUpdate(pos, c, time.Second*5/2)
	}
}

// ScheduledTick kills a live fan that is no longer waterlogged.
func (c CoralWallFan) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if !coralFanWaterlogged(pos, tx) {
		c.Dead = true
		tx.SetBlock(pos, c, nil)
	}
}

// BreakInfo returns the wall fan's breaking properties and floor-fan item drop.
func (c CoralWallFan) BreakInfo() BreakInfo {
	return newBreakInfo(0, alwaysHarvestable, nothingEffective, silkTouchOnlyDrop(CoralFan{Type: c.Type, Dead: c.Dead}))
}

// EncodeItem encodes the wall fan as its shared floor-fan item.
func (c CoralWallFan) EncodeItem() (name string, meta int16) {
	return CoralFan{Type: c.Type, Dead: c.Dead}.EncodeItem()
}

// EncodeBlock encodes the wall fan's distinct identifier and facing state.
func (c CoralWallFan) EncodeBlock() (name string, properties map[string]any) {
	properties = map[string]any{"coral_direction": encodeCoralWallFanDirection(c.Facing)}
	if c.Dead {
		return "minecraft:dead_" + c.Type.String() + "_coral_wall_fan", properties
	}
	return "minecraft:" + c.Type.String() + "_coral_wall_fan", properties
}

// encodeCoralWallFanDirection maps a direction to Bedrock's west/east/north/south ordering.
func encodeCoralWallFanDirection(direction cube.Direction) int32 {
	switch direction {
	case cube.West:
		return 0
	case cube.East:
		return 1
	case cube.North:
		return 2
	default:
		return 3
	}
}

// allCoralWallFans returns every live and dead wall-fan state.
func allCoralWallFans() (fans []world.Block) {
	for _, coralType := range CoralTypes() {
		for _, direction := range cube.Directions() {
			fans = append(fans, CoralWallFan{Type: coralType, Facing: direction})
			fans = append(fans, CoralWallFan{Type: coralType, Dead: true, Facing: direction})
		}
	}
	return
}
