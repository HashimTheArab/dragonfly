package enchantment

import (
	"testing"

	"github.com/df-mc/dragonfly/server/item"
)

func TestRiptide_RegistryIdentity(t *testing.T) {
	id, ok := item.EnchantmentID(Riptide)
	if !ok {
		t.Fatal("Riptide is not registered")
	}
	if id != riptideID {
		t.Fatalf("Riptide ID = %v, want %d", id, riptideID)
	}
	if Riptide.MaxLevel() != 3 {
		t.Fatalf("Riptide max level = %v, want 3", Riptide.MaxLevel())
	}

	got, ok := item.EnchantmentByID(riptideID)
	if !ok || got != Riptide {
		t.Fatalf("enchantment %d = %T, %v, want Riptide, true", riptideID, got, ok)
	}
}

func TestRiptide_FutureExclusiveEnchantmentsAreSymmetric(t *testing.T) {
	for _, id := range []int{loyaltyID, channelingID} {
		other, registered := item.EnchantmentByID(id)
		if !registered {
			continue
		}
		if Riptide.CompatibleWithEnchantment(other) || other.CompatibleWithEnchantment(Riptide) {
			t.Errorf("Riptide compatibility with enchantment %d must be false in both directions", id)
		}
	}
}
