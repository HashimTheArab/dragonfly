package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// TripwireHook is a non-solid block that connects to a tripwire run and emits a redstone signal when an entity
// passes through it.
// TODO: Redstone functionality (connecting to a String run and reacting to it being triggered or cut).
type TripwireHook struct {
	empty
	transparent

	// Facing is the direction the hook points in, away from the block it is attached to.
	Facing cube.Direction
	// Attached is whether the hook is connected to a matching hook through a tripwire run.
	Attached bool
	// Powered is whether the tripwire run the hook is attached to is currently triggered.
	Powered bool
}

// RedstonePower ...
func (t TripwireHook) RedstonePower(cube.Pos, *world.Tx, cube.Face) int {
	if t.Powered {
		return 15
	}
	return 0
}

// RedstoneStrongPower ...
func (t TripwireHook) RedstoneStrongPower(_ cube.Pos, _ *world.Tx, face cube.Face) int {
	if t.Powered && t.Facing.Face() == face {
		return 15
	}
	return 0
}

// NeighbourUpdateTick ...
func (t TripwireHook) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	supportPos := pos.Side(t.Facing.Face().Opposite())
	if !tx.Block(supportPos).Model().FaceSolid(supportPos, t.Facing.Face(), tx) {
		breakBlock(t, pos, tx)
	}
}

// UseOnBlock ...
func (t TripwireHook) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, face, used := firstReplaceable(tx, pos, face, t)
	if !used || face.Axis() == cube.Y {
		return false
	}
	supportPos := pos.Side(face.Opposite())
	if !tx.Block(supportPos).Model().FaceSolid(supportPos, face, tx) {
		return false
	}

	t.Facing, t.Attached, t.Powered = face.Direction(), false, false
	place(tx, pos, t, user, ctx)
	return placed(ctx)
}

// BreakInfo ...
func (t TripwireHook) BreakInfo() BreakInfo {
	return newBreakInfo(0, alwaysHarvestable, nothingEffective, oneOf(TripwireHook{}))
}

// EncodeItem ...
func (t TripwireHook) EncodeItem() (name string, meta int16) {
	return "minecraft:tripwire_hook", 0
}

// EncodeBlock ...
func (t TripwireHook) EncodeBlock() (string, map[string]any) {
	return "minecraft:tripwire_hook", map[string]any{
		"attached_bit": t.Attached,
		"powered_bit":  t.Powered,
		"direction":    encodeTripwireHookDirection(t.Facing),
	}
}

// encodeTripwireHookDirection encodes a direction to the value tripwire hooks use, which starts counting at south
// rather than north.
func encodeTripwireHookDirection(d cube.Direction) int32 {
	switch d {
	case cube.South:
		return 0
	case cube.West:
		return 1
	case cube.North:
		return 2
	}
	return 3
}

// allTripwireHooks returns a list of all tripwire hook variants.
func allTripwireHooks() (hooks []world.Block) {
	for _, d := range cube.Directions() {
		hooks = append(hooks, TripwireHook{Facing: d})
		hooks = append(hooks, TripwireHook{Facing: d, Attached: true})
		hooks = append(hooks, TripwireHook{Facing: d, Powered: true})
		hooks = append(hooks, TripwireHook{Facing: d, Attached: true, Powered: true})
	}
	return
}
