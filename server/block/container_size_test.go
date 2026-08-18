package block

import "testing"

func TestContainerSize(t *testing.T) {
	tests := map[string]struct {
		container ContainerSizer
		want      int
	}{
		"single chest":  {container: NewChest(), want: 27},
		"double chest":  {container: Chest{paired: true}, want: 54},
		"barrel":        {container: NewBarrel(), want: 27},
		"shulker box":   {container: NewShulkerBox(), want: 27},
		"hopper":        {container: NewHopper(), want: 5},
		"ender chest":   {container: NewEnderChest(), want: 27},
		"furnace":       {container: Furnace{}, want: 3},
		"blast furnace": {container: BlastFurnace{}, want: 3},
		"smoker":        {container: Smoker{}, want: 3},
		"brewing stand": {container: BrewingStand{}, want: 5},
		"decorated pot": {container: DecoratedPot{}, want: 1},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := test.container.ContainerSize(); got != test.want {
				t.Fatalf("ContainerSize() = %d, want %d", got, test.want)
			}
		})
	}
}
