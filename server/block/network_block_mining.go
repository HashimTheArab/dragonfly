package block

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/recipe"
	"strings"
)

// effectiveTool derives tool effectiveness from the advertised destructible tags and
// the canonical item-tag registry, rather than keeping another list of tool types.
func (b networkBlock) effectiveTool(tool item.Tool) bool {
	encoded, ok := tool.(interface{ EncodeItem() (string, int16) })
	if !ok {
		return false
	}
	name, _ := encoded.EncodeItem()
	for _, tag := range b.tags {
		if itemTag, ok := strings.CutSuffix(tag, "_item_destructible"); ok && recipe.NewItemTag(itemTag, 1).Contains(name) {
			return true
		}
	}
	return false
}
