package item

import (
	"bytes"
	"maps"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
)

// HasDefaultData reports whether the stack and any original network tags contain
// only the registered item's default data. Count and stack identity are ignored.
// Omitted network defaults are allowed; unrecognised supplied tags are preserved
// by the comparison even when ReadNBT cannot represent them on Stack.
func (s Stack) HasDefaultData(wire map[string]any) bool {
	if s.Empty() {
		return false
	}
	name, metadata := s.Item().EncodeItem()
	plain, ok := world.ItemByName(name, metadata)
	if !ok {
		return false
	}
	defaults := nbt.Compound(WriteNBT(NewStack(plain, 1), false))
	if !defaults.Equal(WriteNBT(s, false)) {
		return false
	}
	if len(wire) == 0 {
		return true
	}
	withWire := maps.Clone(defaults)
	maps.Copy(withWire, wire)
	// Network-decoded empty lists and default nil lists share an NBT encoding.
	var plainData, wireData bytes.Buffer
	plainEncoder, wireEncoder := nbt.NewEncoder(&plainData), nbt.NewEncoder(&wireData)
	plainEncoder.SortMaps, wireEncoder.SortMaps = true, true
	if plainEncoder.Encode(defaults) != nil || wireEncoder.Encode(withWire) != nil {
		return false
	}
	return bytes.Equal(plainData.Bytes(), wireData.Bytes())
}
