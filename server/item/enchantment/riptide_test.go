package enchantment

import (
	"testing"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

func TestRiptide_RegistryIdentity(t *testing.T) {
	id, ok := item.EnchantmentID(Riptide)
	if !ok {
		t.Fatal("Riptide is not registered")
	}
	if id != 30 {
		t.Fatalf("Riptide ID = %v, want 30", id)
	}
	if Riptide.MaxLevel() != 3 {
		t.Fatalf("Riptide max level = %v, want 3", Riptide.MaxLevel())
	}

	got, ok := item.EnchantmentByID(30)
	if !ok || got != Riptide {
		t.Fatalf("enchantment 30 = %T, %v, want Riptide, true", got, ok)
	}
}

func TestRiptide_CompatibleWithItem(t *testing.T) {
	if !Riptide.CompatibleWithItem(testTrident{}) {
		t.Fatal("Riptide is incompatible with a trident")
	}
	if Riptide.CompatibleWithItem(testNonTrident{}) {
		t.Fatal("Riptide is compatible with a non-trident")
	}
}

type testTrident struct{}

func (testTrident) EncodeItem() (string, int16) { return "minecraft:trident", 0 }
func (testTrident) Trident() bool               { return true }

type testNonTrident struct{}

func (testNonTrident) EncodeItem() (string, int16) { return "minecraft:stick", 0 }

var (
	_ world.Item = testTrident{}
	_ world.Item = testNonTrident{}
)
