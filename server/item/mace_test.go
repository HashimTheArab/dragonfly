package item

import "testing"

func TestMace_AttackDamageUsesBedrockTotal(t *testing.T) {
	t.Parallel()

	if got := NewStack(Mace{}, 1).AttackDamage(); got != 7 {
		t.Fatalf("mace attack damage = %v, want 7", got)
	}
}
