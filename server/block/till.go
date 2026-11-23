package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

// TillsInto reports whether using a hoe on from at pos in src makes wanted,
// compared through world.MatchesBuiltState, such as dirt that becomes farmland.
func TillsInto(registry world.BlockRegistry, src world.BlockSource, pos cube.Pos, from, wanted world.Block) bool {
	tillable, ok := from.(item.Tillable)
	if !ok || wanted == nil {
		return false
	}
	result, ok := tillable.Till()
	return ok && world.MatchesBuiltState(registry, src, pos, wanted, result)
}
