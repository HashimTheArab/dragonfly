package block

import (
	"math/rand/v2"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
)

// Button is a non-solid block that emits a full redstone signal for a short time after being pressed. Wooden buttons
// stay pressed half a second longer than stone ones.
type Button struct {
	empty
	transparent
	flowingWaterDisplacer

	// Block is the block to use for the type of button.
	Block world.Block
	// Facing is the face of the supporting block that the button is attached to.
	Facing cube.Face
	// Pressed is whether the button is currently pressed.
	Pressed bool
}

// RedstonePower ...
func (b Button) RedstonePower(cube.Pos, *world.Tx, cube.Face) int {
	if b.Pressed {
		return 15
	}
	return 0
}

// RedstoneStrongPower ...
func (b Button) RedstoneStrongPower(_ cube.Pos, _ *world.Tx, face cube.Face) int {
	if b.Pressed && b.Facing.Opposite() == face {
		return 15
	}
	return 0
}

// SideClosed ...
func (b Button) SideClosed(cube.Pos, cube.Pos, *world.Tx) bool {
	return false
}

// NeighbourUpdateTick ...
func (b Button) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	supportPos := pos.Side(b.Facing.Opposite())
	if !tx.Block(supportPos).Model().FaceSolid(supportPos, b.Facing, tx) {
		breakBlock(b, pos, tx)
	}
}

// UseOnBlock ...
func (b Button) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, face, used := firstReplaceable(tx, pos, face, b)
	if !used {
		return false
	}
	supportPos := pos.Side(face.Opposite())
	if !tx.Block(supportPos).Model().FaceSolid(supportPos, face, tx) {
		return false
	}

	b.Facing, b.Pressed = face, false
	place(tx, pos, b, user, ctx)
	return placed(ctx)
}

// Activate presses the button and schedules the tick that releases it again. Clicking a button that is already
// pressed does nothing, but still counts as an interaction so that no block is placed against it.
func (b Button) Activate(pos cube.Pos, _ cube.Face, tx *world.Tx, _ item.User, _ *item.UseContext) bool {
	if b.Pressed {
		return true
	}
	b.Pressed = true
	tx.SetBlock(pos, b, nil)
	tx.PlaySound(pos.Vec3Centre(), sound.PowerOn{})
	tx.ScheduleBlockUpdate(pos, b, b.pressDuration())
	return true
}

// ScheduledTick ...
func (b Button) ScheduledTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if !b.Pressed {
		return
	}
	b.Pressed = false
	tx.SetBlock(pos, b, nil)
	tx.PlaySound(pos.Vec3Centre(), sound.PowerOff{})
}

// pressDuration returns how long the button stays pressed after being activated.
func (b Button) pressDuration() time.Duration {
	if _, ok := b.Block.(Planks); ok {
		return time.Millisecond * 1500
	}
	return time.Second
}

// BreakInfo ...
func (b Button) BreakInfo() BreakInfo {
	if _, ok := b.Block.(Planks); ok {
		return newBreakInfo(0.5, alwaysHarvestable, axeEffective, oneOf(Button{Block: b.Block}))
	}
	return newBreakInfo(0.5, pickaxeHarvestable, pickaxeEffective, oneOf(Button{Block: b.Block}))
}

// EncodeItem ...
func (b Button) EncodeItem() (name string, meta int16) {
	return "minecraft:" + encodeButtonBlock(b.Block) + "_button", 0
}

// EncodeBlock ...
func (b Button) EncodeBlock() (string, map[string]any) {
	return "minecraft:" + encodeButtonBlock(b.Block) + "_button", map[string]any{
		"button_pressed_bit": b.Pressed,
		"facing_direction":   int32(b.Facing),
	}
}

// ButtonBlocks returns a list of all blocks that buttons may be made of.
func ButtonBlocks() []world.Block {
	blocks := []world.Block{Stone{}, Blackstone{Type: PolishedBlackstone()}}
	for _, w := range WoodTypes() {
		blocks = append(blocks, Planks{Wood: w})
	}
	return blocks
}

// encodeButtonBlock encodes the block passed into the identifier prefix used by its button.
func encodeButtonBlock(block world.Block) string {
	switch block := block.(type) {
	case Planks:
		if block.Wood == OakWood() {
			return "wooden"
		}
		return block.Wood.String()
	case Stone:
		if !block.Smooth {
			return "stone"
		}
	case Blackstone:
		if block.Type == PolishedBlackstone() {
			return block.Type.String()
		}
	}
	panic("invalid block used for button")
}

// allButtons returns a list of all button variants.
func allButtons() (buttons []world.Block) {
	for _, b := range ButtonBlocks() {
		for _, f := range cube.Faces() {
			buttons = append(buttons, Button{Block: b, Facing: f})
			buttons = append(buttons, Button{Block: b, Facing: f, Pressed: true})
		}
	}
	return
}
