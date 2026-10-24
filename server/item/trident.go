package item

// TridentType is implemented by trident items. Dragonfly does not currently
// provide a concrete vanilla trident, but the named contract lets enchantment
// compatibility and custom tridents agree without an anonymous interface.
type TridentType interface {
	Trident() bool
}
