package recipe

import (
	"testing"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/potion"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
)

// brewingTestNetherWart supplies the block-backed reagent without importing block,
// which itself imports recipe. All transformations still come from vanilla data.
type brewingTestNetherWart struct{}

// EncodeItem identifies the vanilla reagent used by the embedded recipes.
func (brewingTestNetherWart) EncodeItem() (string, int16) { return "minecraft:nether_wart", 0 }

// vanillaBrewingRecipes loads only the embedded brewing data, avoiding unrelated
// crafting recipes that require a fully initialised block registry.
func vanillaBrewingRecipes(t *testing.T) []Recipe {
	t.Helper()
	if _, ok := world.ItemByName("minecraft:nether_wart", 0); !ok {
		world.RegisterItem(brewingTestNetherWart{})
	}
	previousRecipes, previousDynamic, previousIndex, previousReagent := recipes, dynamicRecipes, index, reagent
	recipes, dynamicRecipes = nil, nil
	index, reagent = make(map[string]map[string]Recipe), make(map[string]item.Stack)
	t.Cleanup(func() {
		recipes, dynamicRecipes, index, reagent = previousRecipes, previousDynamic, previousIndex, previousReagent
	})
	var data struct {
		Potions          []potionRecipe                `nbt:"potions"`
		ContainerChanges []potionContainerChangeRecipe `nbt:"container_changes"`
	}
	if err := nbt.Unmarshal(vanillaPotionData, &data); err != nil {
		t.Fatal(err)
	}
	for _, entry := range data.Potions {
		input, inputOK := entry.Input.Item()
		reagent, reagentOK := entry.Reagent.Item()
		output, outputOK := entry.Output.Stack()
		if inputOK && reagentOK && outputOK {
			Register(NewPotion(input, reagent, output))
		}
	}
	for _, entry := range data.ContainerChanges {
		input, inputOK := world.ItemByName(entry.Input, 0)
		reagent, reagentOK := entry.Reagent.Item()
		output, outputOK := world.ItemByName(entry.Output, 0)
		stack, stackOK := reagent.(item.Stack)
		if inputOK && reagentOK && outputOK && stackOK {
			Register(NewPotionContainerChange(input, output, stack))
		}
	}
	return Recipes()
}

// assertBrewingItem compares both potion container and variant, including ordinary reagent identities.
func assertBrewingItem(t *testing.T, got, want world.Item) {
	t.Helper()
	gotName, gotMeta := got.EncodeItem()
	wantName, wantMeta := want.EncodeItem()
	if gotName != wantName || gotMeta != wantMeta {
		t.Fatalf("item = %s:%d, want %s:%d", gotName, gotMeta, wantName, wantMeta)
	}
}

func TestBrewingSteps_VanillaRoutes(t *testing.T) {
	available := vanillaBrewingRecipes(t)
	water := item.Potion{Type: potion.Water()}
	for _, test := range []struct {
		name     string
		input    world.Item
		output   world.Item
		reagents []world.Item
	}{
		{"strength", water, item.Potion{Type: potion.Strength()}, []world.Item{brewingTestNetherWart{}, item.BlazePowder{}}},
		{"weakness direct", water, item.Potion{Type: potion.Weakness()}, []world.Item{item.FermentedSpiderEye{}}},
		{"splash strength", item.Potion{Type: potion.Strength()}, item.SplashPotion{Type: potion.Strength()}, []world.Item{item.Gunpowder{}}},
		{"lingering strong strength", item.Potion{Type: potion.StrongStrength()}, item.LingeringPotion{Type: potion.StrongStrength()}, []world.Item{item.Gunpowder{}, item.DragonBreath{}}},
		{"lingering long weakness", item.SplashPotion{Type: potion.LongWeakness()}, item.LingeringPotion{Type: potion.LongWeakness()}, []world.Item{item.DragonBreath{}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			steps, ok := BrewingSteps(test.input, test.output, available)
			if !ok || len(steps) != len(test.reagents) {
				t.Fatalf("BrewingSteps = %v, %v; want %d steps", steps, ok, len(test.reagents))
			}
			current := test.input
			for i, step := range steps {
				assertBrewingItem(t, step.Input, current)
				assertBrewingItem(t, step.Reagent, test.reagents[i])
				result, performed := Perform("brewing_stand", step.Input, step.Reagent)
				if !performed || len(result) != 1 {
					t.Fatalf("step %d cannot be performed: %v, %v", i, result, performed)
				}
				assertBrewingItem(t, step.Output, result[0].Item())
				current = step.Output
			}
			assertBrewingItem(t, current, test.output)
		})
	}
}

func TestBrewingSteps_AvailableCatalog(t *testing.T) {
	water := item.Potion{Type: potion.Water()}
	strength := item.Potion{Type: potion.Strength()}
	custom := NewPotion(item.NewStack(water, 1), item.NewStack(item.Diamond{}, 1), item.NewStack(strength, 1))
	for _, test := range []struct {
		name      string
		input     world.Item
		output    world.Item
		available []Recipe
		want      bool
		count     int
	}{
		{"empty catalog", water, strength, nil, false, 0},
		{"already target", strength, strength, nil, true, 0},
		{"custom route", water, strength, []Recipe{custom}, true, 1},
		{"omitted vanilla route", water, item.Potion{Type: potion.Weakness()}, []Recipe{custom}, false, 0},
		{"non brewing recipe", water, strength, []Recipe{NewShapeless([]Item{item.NewStack(water, 1), item.NewStack(item.Diamond{}, 1)}, item.NewStack(strength, 1), "crafting_table")}, false, 0},
		{"nil input", nil, strength, []Recipe{custom}, false, 0},
		{"nil output", water, nil, []Recipe{custom}, false, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			steps, ok := BrewingSteps(test.input, test.output, test.available)
			if ok != test.want || len(steps) != test.count {
				t.Fatalf("BrewingSteps = %v, %v; want %d steps, %v", steps, ok, test.count, test.want)
			}
			if test.name == "custom route" {
				assertBrewingItem(t, steps[0].Reagent, item.Diamond{})
				assertBrewingItem(t, steps[0].Output, strength)
			}
		})
	}
}

func TestBrewingSteps_UnbrewableOutput(t *testing.T) {
	available := vanillaBrewingRecipes(t)
	for _, output := range []world.Item{item.Potion{Type: potion.Wither()}, item.Diamond{}} {
		steps, ok := BrewingSteps(item.Potion{Type: potion.Water()}, output, available)
		if ok || len(steps) != 0 {
			t.Fatalf("BrewingSteps to %v = %v, %v; want no route", output, steps, ok)
		}
	}
}
