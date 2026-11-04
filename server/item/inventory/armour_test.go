package inventory_test

import (
	"math"
	"testing"

	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/enchantment"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/world"
)

// TestArmour_DamageReductionAppliesProtectionAfterArmour checks observed BDS hits.
func TestArmour_DamageReductionAppliesProtectionAfterArmour(t *testing.T) {
	for _, test := range []struct {
		name         string
		tier         item.ArmourTier
		level        int
		damage, want float64
		mixed        bool
	}{
		{name: "iron IV", tier: item.ArmourTierIron{}, level: 4, damage: 10, want: 2.16},
		{name: "diamond I", tier: item.ArmourTierDiamond{}, level: 1, damage: 10, want: 2.52},
		{name: "mixed", tier: item.ArmourTierIron{}, level: 4, damage: 10, want: 2.304, mixed: true},
		{name: "netherite IV", tier: item.ArmourTierNetherite{}, level: 4, damage: 10, want: 1.008},
		{name: "iron IV 6", tier: item.ArmourTierIron{}, level: 4, damage: 6, want: 1.1232},
		{name: "mixed 13", tier: item.ArmourTierIron{}, level: 4, damage: 13, want: 3.29472, mixed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			pieces := [4]item.Stack{item.NewStack(item.Helmet{Tier: test.tier}, 1), item.NewStack(item.Chestplate{Tier: test.tier}, 1), item.NewStack(item.Leggings{Tier: test.tier}, 1), item.NewStack(item.Boots{Tier: test.tier}, 1)}
			for i := range pieces {
				pieces[i] = pieces[i].WithEnchantments(item.NewEnchantment(enchantment.Protection, test.level))
			}
			if test.mixed {
				pieces[1] = item.NewStack(item.Chestplate{Tier: item.ArmourTierDiamond{}}, 1).WithEnchantments(item.NewEnchantment(enchantment.Protection, 1))
			}
			armour := inventory.NewArmour(nil)
			defer armour.Close()
			armour.Set(pieces[0], pieces[1], pieces[2], pieces[3])
			got := test.damage - armour.DamageReduction(test.damage, entity.AttackDamageSource{})
			if math.Abs(got-test.want) > 1e-8 {
				t.Fatalf("damage=%g, want %g", got, test.want)
			}
		})
	}
}

func TestArmourDamageReduction_DetachedProjectionMatchesEquipped(t *testing.T) {
	pieces := []item.Stack{
		item.NewStack(item.Boots{Tier: item.ArmourTierDiamond{}}, 1).WithEnchantments(item.NewEnchantment(enchantment.FeatherFalling, 4)),
	}
	for _, src := range []world.DamageSource{entity.AttackDamageSource{}, entity.FallDamageSource{}, entity.VoidDamageSource{}} {
		armour := inventory.NewArmour(nil)
		armour.SetBoots(pieces[0])
		want := armour.DamageReduction(10, src)
		got := inventory.ArmourDamageReduction(10, src, append(pieces, item.Stack{})...)
		if got != want {
			t.Fatalf("%T detached=%g equipped=%g", src, got, want)
		}
		armour.Close()
	}
	if got := inventory.ArmourDamageReduction(10, entity.AttackDamageSource{}); got != 0 {
		t.Fatalf("bare damage reduction=%g", got)
	}
	if got := inventory.ArmourDamageReduction(10, entity.FallDamageSource{}, pieces...); math.Abs(got-4.8) > 1e-9 {
		t.Fatalf("fall enchantment without armour=%g, want 4.8", got)
	}
	if got := inventory.ArmourDamageReduction(10, entity.VoidDamageSource{}, pieces...); got != 0 {
		t.Fatalf("void reduction=%g, want 0", got)
	}
}
