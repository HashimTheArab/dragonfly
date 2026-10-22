package block

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

const (
	// BrewingStandIngredientSlot holds the reagent consumed by a brewing step.
	BrewingStandIngredientSlot = 0
	// BrewingStandFirstBottleSlot is the first potion slot in the stand.
	BrewingStandFirstBottleSlot = 1
	// BrewingStandBottleCount is the number of potions brewed together.
	BrewingStandBottleCount = 3
	// BrewingStandFuelSlot holds unused brewing fuel.
	BrewingStandFuelSlot = 4
)

// ContainerSlotSource exposes workstations whose slots use distinct request sources.
// The slot is the inventory slot used by InventoryContent and InventorySlot.
type ContainerSlotSource interface {
	ContainerSlotSourceID(slot int) (byte, bool)
}

// ContainerSlotSourceID resolves a brewing inventory slot's stack-request source.
func (BrewingStand) ContainerSlotSourceID(slot int) (byte, bool) {
	switch {
	case slot == BrewingStandIngredientSlot:
		return protocol.ContainerBrewingStandInput, true
	case slot >= BrewingStandFirstBottleSlot && slot < BrewingStandFirstBottleSlot+BrewingStandBottleCount:
		return protocol.ContainerBrewingStandResult, true
	case slot == BrewingStandFuelSlot:
		return protocol.ContainerBrewingStandFuel, true
	default:
		return 0, false
	}
}

// FuelItem returns the item consumed to replenish a brewing stand's fuel.
func (BrewingStand) FuelItem() world.Item { return item.BlazePowder{} }
