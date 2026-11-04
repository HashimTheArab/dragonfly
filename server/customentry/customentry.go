// Package customentry encodes custom items and blocks as the registry entries a client receives.
package customentry

import (
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
