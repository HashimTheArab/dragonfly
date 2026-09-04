package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// PressurePlate is a non-solid block that emits a redstone signal while entities rest on it. Wooden and stone plates
// emit a full signal, while the weighted plates scale their signal with the number of item entities on them.
// TODO: Entity detection (activation, the deactivation delay and the weighted signal strengths).
type PressurePlate struct {
	empty
	transparent

	// Block is the block to use for the type of pressure plate.
	Block world.Block
	// RedstoneSignal is the signal strength the plate currently emits, between 0 and 15.
	RedstoneSignal int
}

// RedstonePower ...
func (p PressurePlate) RedstonePower(cube.Pos, *world.Tx, cube.Face) int {
	return p.RedstoneSignal
}

// RedstoneStrongPower strongly powers the block the plate rests on.
func (p PressurePlate) RedstoneStrongPower(_ cube.Pos, _ *world.Tx, face cube.Face) int {
	if face != cube.FaceDown {
		return 0
	}
	return p.RedstoneSignal
}

// NeighbourUpdateTick ...
func (p PressurePlate) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	below := pos.Side(cube.FaceDown)
	if !tx.Block(below).Model().FaceSolid(below, cube.FaceUp, tx) {
		breakBlock(p, pos, tx)
	}
}

// UseOnBlock ...
func (p PressurePlate) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) bool {
	pos, _, used := firstReplaceable(tx, pos, face, p)
	if !used {
		return false
	}
	below := pos.Side(cube.FaceDown)
	if !tx.Block(below).Model().FaceSolid(below, cube.FaceUp, tx) {
		return false
	}

	p.RedstoneSignal = 0
	place(tx, pos, p, user, ctx)
	return placed(ctx)
}

// BreakInfo ...
func (p PressurePlate) BreakInfo() BreakInfo {
	if _, ok := p.Block.(Planks); ok {
		return newBreakInfo(0.5, alwaysHarvestable, axeEffective, oneOf(PressurePlate{Block: p.Block}))
	}
	return newBreakInfo(0.5, pickaxeHarvestable, pickaxeEffective, oneOf(PressurePlate{Block: p.Block}))
}

// EncodeItem ...
func (p PressurePlate) EncodeItem() (name string, meta int16) {
	return "minecraft:" + encodePressurePlateBlock(p.Block) + "_pressure_plate", 0
}

// EncodeBlock ...
func (p PressurePlate) EncodeBlock() (string, map[string]any) {
	return "minecraft:" + encodePressurePlateBlock(p.Block) + "_pressure_plate", map[string]any{
		"redstone_signal": int32(p.RedstoneSignal),
	}
}

// PressurePlateBlocks returns a list of all blocks that pressure plates may be made of.
func PressurePlateBlocks() []world.Block {
	blocks := []world.Block{Stone{}, Blackstone{Type: PolishedBlackstone()}, Gold{}, Iron{}}
	for _, w := range WoodTypes() {
		blocks = append(blocks, Planks{Wood: w})
	}
	return blocks
}

// encodePressurePlateBlock encodes the block passed into the identifier prefix used by its pressure plate.
func encodePressurePlateBlock(block world.Block) string {
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
	case Gold:
		return "light_weighted"
	case Iron:
		return "heavy_weighted"
	}
	panic("invalid block used for pressure plate")
}

// allPressurePlates returns a list of all pressure plate variants.
func allPressurePlates() (plates []world.Block) {
	for _, b := range PressurePlateBlocks() {
		for signal := 0; signal <= 15; signal++ {
			plates = append(plates, PressurePlate{Block: b, RedstoneSignal: signal})
		}
	}
	return
}
