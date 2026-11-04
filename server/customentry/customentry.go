// Package customentry encodes custom items and blocks as the registry entries a client receives.
package customentry

import (
	"slices"
	"strings"

	"github.com/df-mc/dragonfly/server/internal/blockinternal"
	"github.com/df-mc/dragonfly/server/internal/iteminternal"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// Item creates the protocol entry sent to a client for it. The runtimeID must
// be unique among the item entries in the same registry.
//
// Items that are also custom blocks are placed by the block they encode, so they carry no
// item components of their own and use ItemEntryVersionNone.
func Item(it world.CustomItem, runtimeID int16) protocol.ItemEntry {
	name, _ := it.EncodeItem()
	_, isCustomBlock := it.(world.CustomBlock)
	version := int32(protocol.ItemEntryVersionDataDriven)
	if isCustomBlock {
		version = protocol.ItemEntryVersionNone
	}
	return protocol.ItemEntry{
		Name:           name,
		ComponentBased: !isCustomBlock,
		RuntimeID:      runtimeID,
		Version:        version,
		Data:           iteminternal.Components(it),
	}
}

// Block creates the protocol entry sent to a client for b. The
// blockID must be unique among the custom block entries in the same palette.
func Block(b world.CustomBlock, blockID int32) protocol.BlockEntry {
	name, _ := b.EncodeBlock()
	return protocol.BlockEntry{
		Name:       name,
		Properties: blockinternal.Components(name, b, blockID),
	}
}

// DeclareNetworkHashSalt adds world.NetworkHashSaltProperty, with salt as its one value, to the properties of entry,
// a custom block's entry. A registry with hashed network IDs salts a block whose network block hashes would collide
// (world.BasicBlockRegistry.NetworkHashSalt), and the client hashes the block's states from the properties its entry
// declares. One value adds no states and leaves their order unchanged. Properties stay sorted by name.
func DeclareNetworkHashSalt(entry *protocol.BlockEntry, salt int32) {
	properties, _ := entry.Properties["properties"].([]map[string]any)
	property := map[string]any{"name": world.NetworkHashSaltProperty, "enum": []any{salt}}
	i, _ := slices.BinarySearchFunc(properties, world.NetworkHashSaltProperty, func(p map[string]any, name string) int {
		n, _ := p["name"].(string)
		return strings.Compare(n, name)
	})
	entry.Properties["properties"] = slices.Insert(properties, i, property)
}
