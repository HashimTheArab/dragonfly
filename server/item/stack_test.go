package item

import "testing"

func TestRepeatStacks(t *testing.T) {
	// A sword stacks to 1, so each repetition needs its own stack.
	swords := RepeatStacks([]Stack{NewStack(Sword{Tier: ToolTierDiamond}, 1)}, 3)
	if len(swords) != 3 {
		t.Fatalf("3 swords produced %d stacks, want 3", len(swords))
	}

	// 20 of a 64-stacking item repeated 4 times is 80: one full stack and a remainder.
	dirt := RepeatStacks([]Stack{NewStack(testItem{}, 20)}, 4)
	if len(dirt) != 2 {
		t.Fatalf("80 items produced %d stacks, want 2", len(dirt))
	}
	if got := dirt[0].Count() + dirt[1].Count(); got != 80 {
		t.Fatalf("stacks hold %d items in total, want 80", got)
	}
	if dirt[0].Count() != 64 {
		t.Fatalf("first stack holds %d, want it filled to 64", dirt[0].Count())
	}
}

type testItem struct{}

func (testItem) EncodeItem() (string, int16) { return "minecraft:test", 0 }

// TestStack_MaximumDamageRetainsLastUse covers the stone pickaxe that BDS
// still sends at Damage=131, including its network NBT round trip.
func TestStack_MaximumDamageRetainsLastUse(t *testing.T) {
	for _, tier := range ToolTiers() {
		t.Run(tier.Name, func(t *testing.T) {
			fresh := NewStack(Pickaxe{Tier: tier}, 1)
			tag := map[string]any{"Damage": int32(tier.Durability)}
			last := ReadNBT(tag, &fresh)
			if last.Empty() || last.Durability() != 0 {
				t.Fatalf("maximum damage discarded last use: count=%d durability=%d", last.Count(), last.Durability())
			}
			if got := WriteNBT(last, false)["Damage"]; got != int32(tier.Durability) {
				t.Fatalf("round trip damage=%v, want %d", got, tier.Durability)
			}
			if !last.Damage(1).Empty() {
				t.Fatal("use beyond maximum damage did not break tool")
			}
			if got := fresh.WithDurability(0); got.Empty() || got.Durability() != 0 {
				t.Fatal("zero durability discarded last use")
			}
			if !fresh.WithDurability(-1).Empty() {
				t.Fatal("negative durability retained broken tool")
			}
		})
	}
}
