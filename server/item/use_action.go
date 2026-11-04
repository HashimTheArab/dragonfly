package item

import "github.com/df-mc/dragonfly/server/world"

// HasUseAction reports whether it declares a native placement, activation,
// consumption or held-use action. It classifies capabilities, independently of
// whether that action is currently possible for a particular user or target.
func HasUseAction(it world.Item) bool {
	switch it.(type) {
	case world.Block, UsableOnBlock, UsableOnEntity, Usable, Consumable, Chargeable, Releasable:
		return true
	default:
		return false
	}
}
