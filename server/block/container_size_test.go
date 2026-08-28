package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

type pairedContainerSource struct {
	blocks map[cube.Pos]world.Block
	data   map[cube.Pos]map[string]any
}

// Block returns the block stored at pos.
func (s pairedContainerSource) Block(pos cube.Pos) world.Block {
	if b, ok := s.blocks[pos]; ok {
		return b
	}
	return Air{}
}

// BlockEntityData returns the block-entity data stored at pos.
func (s pairedContainerSource) BlockEntityData(pos cube.Pos) (map[string]any, bool) {
	data, ok := s.data[pos]
	return data, ok
}

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

func TestIsValidPairedContainerAt(t *testing.T) {
	pos, pair := cube.Pos{1, 64, 1}, cube.Pos{2, 64, 1}
	source := pairedContainerSource{
		blocks: map[cube.Pos]world.Block{pos: NewChest(), pair: NewChest()},
		data: map[cube.Pos]map[string]any{
			pos:  {"pairx": int32(pair[0]), "pairz": int32(pair[2])},
			pair: {"pairx": int32(pos[0]), "pairz": int32(pos[2])},
		},
	}
	if !IsValidPairedContainerAt(source, pos) || !IsValidPairedContainerAt(source, pair) {
		t.Fatal("reciprocal matching chests were not accepted")
	}
	first, second := NewChest(), NewChest()
	first.Facing, second.Facing = cube.North, cube.South
	source.blocks[pos], source.blocks[pair] = first, second
	if IsValidPairedContainerAt(source, pos) {
		t.Fatal("reciprocal chests with different facings were accepted")
	}
	source.blocks[pos], source.blocks[pair] = NewChest(), NewChest()
	source.data[pair] = map[string]any{"pairx": int32(9), "pairz": int32(9)}
	if IsValidPairedContainerAt(source, pos) {
		t.Fatal("one-sided pair metadata was accepted")
	}
}

func TestContainerSizeAt(t *testing.T) {
	pos, pair := cube.Pos{1, 64, 1}, cube.Pos{2, 64, 1}
	source := pairedContainerSource{
		blocks: map[cube.Pos]world.Block{pos: NewChest(), pair: NewChest()},
		data: map[cube.Pos]map[string]any{
			pos:  {"pairx": int32(pair[0]), "pairz": int32(pair[2])},
			pair: {"pairx": int32(pos[0]), "pairz": int32(pos[2])},
		},
	}
	if got, ok := ContainerSizeAt(source, pos); !ok || got != 54 {
		t.Fatalf("paired chest size = %d, %t; want 54, true", got, ok)
	}
	source.data = nil
	if got, ok := ContainerSizeAt(source, pos); !ok || got != 27 {
		t.Fatalf("single chest size = %d, %t; want 27, true", got, ok)
	}
	source.blocks[pos] = Stone{}
	if got, ok := ContainerSizeAt(source, pos); ok || got != 0 {
		t.Fatalf("non-container size = %d, %t; want 0, false", got, ok)
	}
}
