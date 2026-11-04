package item

import "testing"

func TestMace_AttackDamageUsesBedrockTotal(t *testing.T) {
	t.Parallel()

	// Bedrock's unenchanted normal attack deals 6 total damage. Stack adds the
	// base point to Weapon.AttackDamage, so Mace supplies the remaining 5.
	// https://minecraft.wiki/w/Mace#Bedrock_Edition
	if got := NewStack(Mace{}, 1).AttackDamage(); got != 6 {
		t.Fatalf("mace attack damage = %v, want 6", got)
	}
}
