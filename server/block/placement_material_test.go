package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

// TestPlacementItem_CountsBlockParts distinguishes build quantities from drops.
func TestPlacementItem_CountsBlockParts(t *testing.T) {
	for _, test := range []struct {
		name      string
		block     world.Block
		wantName  string
		wantCount int
	}{
		{"air", Air{}, "minecraft:air", 0},
		{"log", Log{Wood: OakWood(), Axis: cube.X}, "minecraft:oak_log", 1},
		{"slab", Slab{Block: Planks{Wood: OakWood()}}, "minecraft:oak_slab", 1},
		{"double slab", Slab{Block: Planks{Wood: OakWood()}, Double: true}, "minecraft:oak_slab", 2},
		{"wood door bottom", WoodDoor{Wood: OakWood()}, "minecraft:wooden_door", 1},
		{"wood door top", WoodDoor{Wood: OakWood(), Top: true}, "minecraft:wooden_door", 0},
		{"iron door bottom", IronDoor{}, "minecraft:iron_door", 1},
		{"iron door top", IronDoor{Top: true}, "minecraft:iron_door", 0},
		{"copper door bottom", CopperDoor{}, "minecraft:copper_door", 1},
		{"copper door top", CopperDoor{Top: true}, "minecraft:copper_door", 0},
		{"bed foot", Bed{Colour: item.ColourRed()}, "minecraft:bed", 1},
		{"bed head", Bed{Head: true, Colour: item.ColourRed()}, "minecraft:bed", 0},
		{"flower lower", DoubleFlower{}, "minecraft:sunflower", 1},
		{"flower upper", DoubleFlower{UpperPart: true}, "minecraft:sunflower", 0},
		{"grass lower", DoubleTallGrass{}, "minecraft:tall_grass", 1},
		{"grass upper", DoubleTallGrass{UpperPart: true}, "minecraft:tall_grass", 0},
		{"one candle", Candle{}, "minecraft:candle", 1},
		{"four candles", Candle{AdditionalCandles: 3}, "minecraft:candle", 4},
		{"one pickle", SeaPickle{}, "minecraft:sea_pickle", 1},
		{"four pickles", SeaPickle{AdditionalCount: 3}, "minecraft:sea_pickle", 4},
		{"dust", RedstoneWire{Power: 15}, "minecraft:redstone", 1},
		{"unlit torch", RedstoneTorch{Facing: cube.FaceDown}, "minecraft:redstone_torch", 1},
		{"lit furnace", Furnace{Lit: true}, "minecraft:furnace", 1},
		{"lit blast furnace", BlastFurnace{Lit: true}, "minecraft:blast_furnace", 1},
		{"lit smoker", Smoker{Lit: true}, "minecraft:smoker", 1},
		{"water", Water{Still: true, Depth: 8}, "", 1},
		{"flowing water", Water{Depth: 4}, "", 1},
		{"lava", Lava{Still: true, Depth: 8}, "", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			it, count := world.PlacementItem(test.block)
			if count != test.wantCount {
				t.Fatalf("count = %d, want %d", count, test.wantCount)
			}
			if test.wantName == "" {
				if it != nil {
					t.Fatalf("unexpected item: %T", it)
				}
				return
			}
			if it == nil {
				t.Fatal("missing placement item")
			}
			if name, _ := it.EncodeItem(); name != test.wantName {
				t.Fatalf("item = %s, want %s", name, test.wantName)
			}
			if test.name == "bed foot" {
				if _, meta := it.EncodeItem(); meta != int16(item.ColourRed().Uint8()) {
					t.Fatalf("bed colour metadata = %d", meta)
				}
			}
		})
	}
}

// TestPlacementItem_MetadataSurvivesRegistryRoundTrip checks every metadata-only
// state through the public registry, cloning and recording paths.
func TestPlacementItem_MetadataSurvivesRegistryRoundTrip(t *testing.T) {
	registry := world.NewBlockRegistry()
	registry.RegisterBlockState(world.BlockState{Name: "example:unknown"})
	registry.Finalize()
	snapshot, err := world.MarshalBlockRegistry(registry)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := world.UnmarshalBlockRegistry(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	states := append(allRepeaters(), allComparators()...)
	states = append(states, allSnowLayers()...)
	states = append(states, allTurtleEggs()...)
	for _, r := range []world.BlockRegistry{registry, registry.(*world.BasicBlockRegistry).Clone(), restored} {
		for _, state := range states {
			name, properties := state.EncodeBlock()
			b, ok := r.BlockByName(name, properties)
			if !ok {
				t.Fatalf("state missing: %s %v", name, properties)
			}
			if !world.IsUnknownBlock(b) {
				t.Fatalf("metadata installed gameplay behaviour for %s", name)
			}
			it, count := world.PlacementItem(b)
			if it == nil {
				t.Fatalf("missing item: %s %v", name, properties)
			}
			wantName, wantCount := "", 1
			switch name {
			case "minecraft:powered_repeater", "minecraft:unpowered_repeater":
				wantName = "minecraft:repeater"
			case "minecraft:powered_comparator", "minecraft:unpowered_comparator":
				wantName = "minecraft:comparator"
			case "minecraft:snow_layer":
				wantName, wantCount = name, int(properties["height"].(int32))+1
			case "minecraft:turtle_egg":
				wantName = name
				wantCount = map[string]int{"one_egg": 1, "two_egg": 2, "three_egg": 3, "four_egg": 4}[properties["turtle_egg_count"].(string)]
			default:
				t.Fatalf("unexpected metadata state: %s", name)
			}
			gotName, meta := it.EncodeItem()
			if gotName != wantName || meta != 0 || count != wantCount {
				t.Fatalf("%s %v: item=%s:%d count=%d, want %s:0 count=%d", name, properties, gotName, meta, count, wantName, wantCount)
			}
		}
		b, ok := r.BlockByName("example:unknown", nil)
		if !ok {
			t.Fatal("custom unknown state missing")
		}
		if it, count := world.PlacementItem(b); it != nil || count != 1 {
			t.Fatalf("unknown material = %v %d", it, count)
		}
	}
}
