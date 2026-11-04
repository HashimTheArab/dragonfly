package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
)

// RespawnAnchor is a block that stores up to four glowstone charges and lights up brighter with every one of them.
// TODO: Using a charged anchor (setting the player's spawn in the nether and exploding anywhere else).
type RespawnAnchor struct {
	solid
	bassDrum

	// Charge is the amount of glowstone charges the anchor holds, between 0 and 4.
	Charge int
}

// LightEmissionLevel ...
func (r RespawnAnchor) LightEmissionLevel() uint8 {
	if r.Charge == 0 {
		return 0
	}
	return uint8(r.Charge*4 - 1)
}

// Activate charges the anchor with the glowstone the user is holding.
func (r RespawnAnchor) Activate(pos cube.Pos, _ cube.Face, tx *world.Tx, u item.User, ctx *item.UseContext) bool {
	if r.Charge >= 4 {
		return false
	}
	held, _ := u.HeldItems()
	if _, ok := held.Item().(Glowstone); !ok {
		return false
	}

	r.Charge++
	tx.SetBlock(pos, r, nil)
	tx.PlaySound(pos.Vec3Centre(), sound.Custom{Name: "respawn_anchor.charge", Volume: 1, Pitch: 1})
	ctx.SubtractFromCount(1)
	return true
}

// BreakInfo ...
func (r RespawnAnchor) BreakInfo() BreakInfo {
	return newBreakInfo(50, func(t item.Tool) bool {
		return t.ToolType() == item.TypePickaxe && t.HarvestLevel() >= item.ToolTierDiamond.HarvestLevel
	}, pickaxeEffective, oneOf(RespawnAnchor{})).withBlastResistance(1200)
}

// EncodeItem ...
func (r RespawnAnchor) EncodeItem() (name string, meta int16) {
	return "minecraft:respawn_anchor", 0
}

// EncodeBlock ...
func (r RespawnAnchor) EncodeBlock() (string, map[string]any) {
	return "minecraft:respawn_anchor", map[string]any{"respawn_anchor_charge": int32(r.Charge)}
}

// allRespawnAnchors returns a list of all respawn anchor variants.
func allRespawnAnchors() (anchors []world.Block) {
	for charge := 0; charge <= 4; charge++ {
		anchors = append(anchors, RespawnAnchor{Charge: charge})
	}
	return
}
