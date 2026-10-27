package block

import "github.com/sandertv/gophertunnel/minecraft/protocol"

const (
	// SmelterInputSlot holds the item a furnace, blast furnace or smoker smelts.
	SmelterInputSlot = 0
	// SmelterFuelSlot holds the fuel burnt to smelt the input.
	SmelterFuelSlot = 1
	// SmelterResultSlot holds the smelted product.
	SmelterResultSlot = 2
)

// smelterSlotSourceID resolves a smelter inventory slot's stack-request source. Only
// the input slot is workstation-specific; fuel and result share the furnace sources.
func smelterSlotSourceID(slot int, input byte) (byte, bool) {
	switch slot {
	case SmelterInputSlot:
		return input, true
	case SmelterFuelSlot:
		return protocol.ContainerFurnaceFuel, true
	case SmelterResultSlot:
		return protocol.ContainerFurnaceResult, true
	default:
		return 0, false
	}
}

// ContainerSlotSourceID resolves a furnace inventory slot's stack-request source.
func (Furnace) ContainerSlotSourceID(slot int) (byte, bool) {
	return smelterSlotSourceID(slot, protocol.ContainerFurnaceIngredient)
}

// ContainerSlotSourceID resolves a blast furnace inventory slot's stack-request source.
func (BlastFurnace) ContainerSlotSourceID(slot int) (byte, bool) {
	return smelterSlotSourceID(slot, protocol.ContainerBlastFurnaceIngredient)
}

// ContainerSlotSourceID resolves a smoker inventory slot's stack-request source.
func (Smoker) ContainerSlotSourceID(slot int) (byte, bool) {
	return smelterSlotSourceID(slot, protocol.ContainerSmokerIngredient)
}
