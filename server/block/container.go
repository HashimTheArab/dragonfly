package block

import (
	"strings"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/world"
)

const (
	// PairableChestName is the block identifier of a normal pairable chest.
	PairableChestName = "minecraft:chest"
	// PairableTrappedChestName is the block identifier of a trapped pairable chest.
	PairableTrappedChestName = "minecraft:trapped_chest"
)

// PairableChestNames lists the standard chest block identifiers that may pair.
var PairableChestNames = [...]string{PairableChestName, PairableTrappedChestName}

// ContainerViewer represents a viewer that is able to view a container and its inventory.
type ContainerViewer interface {
	world.Viewer
	// ViewSlotChange views a change of a single slot in the inventory, in which the item was changed to the
	// new item passed.
	ViewSlotChange(slot int, newItem item.Stack)
}

// ContainerOpener represents an entity that is able to open a container.
type ContainerOpener interface {
	// OpenBlockContainer opens a block container at the position passed.
	OpenBlockContainer(pos cube.Pos, tx *world.Tx)
}

// Container represents a container of items, typically a block such as a chest. Containers may have their
// inventory opened by viewers.
type Container interface {
	AddViewer(v ContainerViewer, tx *world.Tx, pos cube.Pos)
	RemoveViewer(v ContainerViewer, tx *world.Tx, pos cube.Pos)
	Inventory(tx *world.Tx, pos cube.Pos) *inventory.Inventory
	ContainerSize() int
}

// ContainerSizer exposes the client-visible number of slots of a container
// block without requiring a world transaction. It is useful to consumers that
// mirror an already-loaded block state but do not own the server world.
type ContainerSizer interface {
	ContainerSize() int
}

// PairedContainerSource exposes the loaded block and block-entity state needed
// to validate a paired container.
type PairedContainerSource interface {
	Block(cube.Pos) world.Block
	BlockEntityData(cube.Pos) (map[string]any, bool)
}

// ContainerSizeAt returns the client-visible capacity of the container at pos.
// Paired chest models expose the combined capacity of both physical blocks.
func ContainerSizeAt(source PairedContainerSource, pos cube.Pos) (int, bool) {
	b := source.Block(pos)
	sizer, ok := b.(ContainerSizer)
	if !ok {
		return 0, false
	}
	size := sizer.ContainerSize()
	if size <= 0 {
		return 0, false
	}
	name, _ := b.EncodeBlock()
	if size == 27 && IsPairedContainerName(name) && IsValidPairedContainerAt(source, pos) {
		return 54, true
	}
	return size, true
}

// IsPairableChestName reports whether name is a standard chest that may pair.
func IsPairableChestName(name string) bool {
	return name == PairableChestName || name == PairableTrappedChestName
}

// IsPairedContainerName reports whether name uses the paired 27/54-slot chest model.
func IsPairedContainerName(name string) bool {
	return IsPairableChestName(name) || strings.HasSuffix(name, "copper_chest")
}

// PairedContainerPosition returns the paired position encoded in chest block-entity data.
func PairedContainerPosition(data map[string]any, y int) (cube.Pos, bool) {
	x, xOK := data["pairx"].(int32)
	z, zOK := data["pairz"].(int32)
	return cube.Pos{int(x), y, int(z)}, xOK && zOK
}

// IsValidPairedContainerAt reports whether pos and its referenced partner have
// matching block types and reciprocal pair metadata.
func IsValidPairedContainerAt(source PairedContainerSource, pos cube.Pos) bool {
	name, properties := source.Block(pos).EncodeBlock()
	if !IsPairedContainerName(name) {
		return false
	}
	data, ok := source.BlockEntityData(pos)
	if !ok {
		return false
	}
	pairPos, ok := PairedContainerPosition(data, pos[1])
	if !ok {
		return false
	}
	dx, dz := pairPos[0]-pos[0], pairPos[2]-pos[2]
	if dx*dx+dz*dz != 1 {
		return false
	}
	pairName, pairProperties := source.Block(pairPos).EncodeBlock()
	if pairName != name || !matchingPairedContainerFacing(properties, pairProperties) {
		return false
	}
	pairData, ok := source.BlockEntityData(pairPos)
	if !ok {
		return false
	}
	back, ok := PairedContainerPosition(pairData, pairPos[1])
	return ok && back == pos
}

// matchingPairedContainerFacing reports whether two paired-container block
// states declare the same cardinal direction.
func matchingPairedContainerFacing(a, b map[string]any) bool {
	aValue, aPresent := a["minecraft:cardinal_direction"]
	bValue, bPresent := b["minecraft:cardinal_direction"]
	if aPresent != bPresent {
		return false
	}
	if !aPresent {
		return true
	}
	aFacing, aOK := aValue.(string)
	bFacing, bOK := bValue.(string)
	return aOK && bOK && aFacing == bFacing
}
