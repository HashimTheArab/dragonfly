package item

import "testing"

type customArmourSlotItem struct{}

// EncodeItem gives the fixture a stable item identity.
func (customArmourSlotItem) EncodeItem() (string, int16) { return "test:custom_armour", 0 }

// ArmourSlot returns the fixture's custom armour slot.
func (customArmourSlotItem) ArmourSlot() (int, bool) { return 4, true }

type customBodyArmourItem struct{ customArmourSlotItem }

// ArmourSlot leaves the custom slot unspecified so native body capability applies.
func (customBodyArmourItem) ArmourSlot() (int, bool) { return 0, false }

// Body reports that the fixture is body armour.
func (customBodyArmourItem) Body() bool { return true }

func TestArmourSlotProvider(t *testing.T) {
	if slot, ok := ArmourSlot(customArmourSlotItem{}); !ok || slot != 4 {
		t.Fatalf("ArmourSlot() = %d, %t; want 4, true", slot, ok)
	}
	if slot, ok := ArmourSlot(customBodyArmourItem{}); !ok || slot != 4 {
		t.Fatalf("body ArmourSlot() = %d, %t; want 4, true", slot, ok)
	}
}
