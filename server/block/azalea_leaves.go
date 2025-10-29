package block

import (
	"math/rand"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// AzaleaLeaves are blocks that grow as part of trees which mainly drop saplings and sticks.
type AzaleaLeaves struct {
	leaves
	sourceWaterDisplacer

	// Flowering specifies if this block is flowering.
	Flowering bool

	// Persistent specifies if the leaves are persistent, meaning they will not decay as a result of no wood
	// being nearby.
	Persistent bool

	ShouldUpdate bool
}

// UseOnBlock makes leaves persistent when they are placed so that they don't decay.
func (l AzaleaLeaves) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, tx *world.Tx, user item.User, ctx *item.UseContext) (used bool) {
	pos, _, used = firstReplaceable(tx, pos, face, l)
	if !used {
		return
	}
	l.Persistent = true

	place(tx, pos, l, user, ctx)
	return placed(ctx)
}

// RandomTick ...
func (l AzaleaLeaves) RandomTick(pos cube.Pos, tx *world.Tx, _ *rand.Rand) {
	if !l.Persistent && l.ShouldUpdate {
		if findLog(pos, tx, &[]cube.Pos{}, 0) {
			l.ShouldUpdate = false
			tx.SetBlock(pos, l, nil)
		} else {
			tx.SetBlock(pos, nil, nil)
		}
	}
}

// NeighbourUpdateTick ...
func (l AzaleaLeaves) NeighbourUpdateTick(pos, _ cube.Pos, tx *world.Tx) {
	if !l.Persistent && !l.ShouldUpdate {
		l.ShouldUpdate = true
		tx.SetBlock(pos, l, nil)
	}
}

// FlammabilityInfo ...
func (l AzaleaLeaves) FlammabilityInfo() FlammabilityInfo {
	return newFlammabilityInfo(30, 60, true)
}

// BreakInfo ...
func (l AzaleaLeaves) BreakInfo() BreakInfo {
	return newBreakInfo(0.2, alwaysHarvestable, func(t item.Tool) bool {
		return t.ToolType() == item.TypeShears || t.ToolType() == item.TypeHoe
	}, func(t item.Tool, enchantments []item.Enchantment) []item.Stack {
		if t.ToolType() == item.TypeShears || hasSilkTouch(enchantments) {
			return []item.Stack{item.NewStack(l, 1)}
		}
		var drops []item.Stack
		if rand.Float64() < 0.005 {
			drops = append(drops, item.NewStack(item.Apple{}, 1))
		}
		// TODO: Saplings and sticks can drop along with apples
		return drops
	})
}

// CompostChance ...
func (AzaleaLeaves) CompostChance() float64 {
	return 0.3
}

// EncodeItem ...
func (l AzaleaLeaves) EncodeItem() (name string, meta int16) {
	name = "minecraft:azalea_leaves"
	if l.Flowering {
		name += "_flowered"
	}
	return name, 0
}

// LightDiffusionLevel ...
func (AzaleaLeaves) LightDiffusionLevel() uint8 {
	return 1
}

// SideClosed ...
func (AzaleaLeaves) SideClosed(cube.Pos, cube.Pos, *world.World) bool {
	return false
}

// EncodeBlock ...
func (l AzaleaLeaves) EncodeBlock() (name string, properties map[string]any) {
	name = "minecraft:azalea_leaves"
	if l.Flowering {
		name += "_flowered"
	}
	return name, map[string]any{"persistent_bit": l.Persistent, "update_bit": l.ShouldUpdate}
}

// allLogs returns a list of all possible leaves states.
func allAzaleaLeaves() (leaves []world.Block) {
	f := func(persistent, update, flowering bool) {
		leaves = append(leaves, AzaleaLeaves{Flowering: flowering, Persistent: persistent, ShouldUpdate: update})
	}
	f(true, true, true)
	f(true, false, true)
	f(false, true, true)
	f(false, false, true)
	f(true, true, false)
	f(true, false, false)
	f(false, true, false)
	f(false, false, false)
	return leaves
}
