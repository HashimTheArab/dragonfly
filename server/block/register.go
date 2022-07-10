package block

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

//go:generate go run ../../cmd/blockhash -o hash.go .

// init registers all blocks implemented by Dragonfly.
func init() {
	world.RegisterBlock(Air{})
	world.RegisterBlock(Amethyst{})
	world.RegisterBlock(AncientDebris{})
	world.RegisterBlock(Andesite{Polished: true})
	world.RegisterBlock(Andesite{})
	world.RegisterBlock(Barrier{})
	world.RegisterBlock(Beacon{})
	world.RegisterBlock(Bedrock{InfiniteBurning: true})
	world.RegisterBlock(Bedrock{})
	world.RegisterBlock(BlueIce{})
	world.RegisterBlock(Bookshelf{})
	world.RegisterBlock(Bricks{})
	world.RegisterBlock(Calcite{})
	world.RegisterBlock(Clay{})
	world.RegisterBlock(Coal{})
	world.RegisterBlock(Cobblestone{Mossy: true})
	world.RegisterBlock(Cobblestone{})
	world.RegisterBlock(CraftingTable{})
	world.RegisterBlock(DeadBush{})
	world.RegisterBlock(Diamond{})
	world.RegisterBlock(Diorite{Polished: true})
	world.RegisterBlock(Diorite{})
	world.RegisterBlock(DirtPath{})
	world.RegisterBlock(Dirt{Coarse: true})
	world.RegisterBlock(Dirt{})
	world.RegisterBlock(DragonEgg{})
	world.RegisterBlock(DriedKelp{})
	world.RegisterBlock(Dripstone{})
	world.RegisterBlock(Emerald{})
	world.RegisterBlock(EndBricks{})
	world.RegisterBlock(EndStone{})
	world.RegisterBlock(GildedBlackstone{})
	world.RegisterBlock(GlassPane{})
	world.RegisterBlock(Glass{})
	world.RegisterBlock(Glowstone{})
	world.RegisterBlock(Gold{})
	world.RegisterBlock(Granite{Polished: true})
	world.RegisterBlock(Granite{})
	world.RegisterBlock(Grass{})
	world.RegisterBlock(Gravel{})
	world.RegisterBlock(Honeycomb{})
	world.RegisterBlock(InvisibleBedrock{})
	world.RegisterBlock(IronBars{})
	world.RegisterBlock(Iron{})
	world.RegisterBlock(Lapis{})
	world.RegisterBlock(Melon{})
	world.RegisterBlock(MossCarpet{})
	world.RegisterBlock(MudBricks{})
	world.RegisterBlock(MuddyMangroveRoots{})
	world.RegisterBlock(Mud{})
	world.RegisterBlock(NetherBrickFence{})
	world.RegisterBlock(NetherGoldOre{})
	world.RegisterBlock(NetherQuartzOre{})
	world.RegisterBlock(NetherSprouts{})
	world.RegisterBlock(Netherite{})
	world.RegisterBlock(Netherrack{})
	world.RegisterBlock(Note{})
	world.RegisterBlock(Obsidian{Crying: true})
	world.RegisterBlock(Obsidian{})
	world.RegisterBlock(PackedIce{})
	world.RegisterBlock(PackedMud{})
	world.RegisterBlock(Podzol{})
	world.RegisterBlock(QuartzBricks{})
	world.RegisterBlock(RawCopper{})
	world.RegisterBlock(RawGold{})
	world.RegisterBlock(RawIron{})
	world.RegisterBlock(ReinforcedDeepslate{})
	world.RegisterBlock(Sand{Red: true})
	world.RegisterBlock(Sand{})
	world.RegisterBlock(SeaLantern{})
	world.RegisterBlock(Shroomlight{})
	world.RegisterBlock(Snow{})
	world.RegisterBlock(SoulSand{})
	world.RegisterBlock(SoulSoil{})
	world.RegisterBlock(Sponge{Wet: true})
	world.RegisterBlock(Sponge{})
	world.RegisterBlock(SporeBlossom{})
	world.RegisterBlock(Stone{Smooth: true})
	world.RegisterBlock(Stone{})
	world.RegisterBlock(Terracotta{})
	world.RegisterBlock(Tuff{})

	for _, ore := range OreTypes() {
		world.RegisterBlock(CoalOre{Type: ore})
		world.RegisterBlock(CopperOre{Type: ore})
		world.RegisterBlock(DiamondOre{Type: ore})
		world.RegisterBlock(EmeraldOre{Type: ore})
		world.RegisterBlock(GoldOre{Type: ore})
		world.RegisterBlock(IronOre{Type: ore})
		world.RegisterBlock(LapisOre{Type: ore})
	}

	registerAll(allBarrels())
	registerAll(allBasalt())
	registerAll(allBeetroot())
	registerAll(allBoneBlock())
	registerAll(allCactus())
	registerAll(allCake())
	registerAll(allCarpet())
	registerAll(allCarrots())
	registerAll(allChains())
	registerAll(allChests())
	registerAll(allCobblestoneStairs())
	registerAll(allCocoaBeans())
	registerAll(allConcrete())
	registerAll(allConcretePowder())
	registerAll(allCoral())
	registerAll(allCoralBlocks())
	registerAll(allDoors())
	registerAll(allDoubleFlowers())
	registerAll(allDoubleTallGrass())
	registerAll(allEndBrickStairs())
	registerAll(allFarmland())
	registerAll(allFence())
	registerAll(allFenceGates())
	registerAll(allFire())
	registerAll(allFlowers())
	registerAll(allFroglight())
	registerAll(allGlazedTerracotta())
	registerAll(allItemFrames())
	registerAll(allKelp())
	registerAll(allLadders())
	registerAll(allLanterns())
	registerAll(allLava())
	registerAll(allLeaves())
	registerAll(allLight())
	registerAll(allLitPumpkins())
	registerAll(allLogs())
	registerAll(allMelonStems())
	registerAll(allNetherBricks())
	registerAll(allNetherWart())
	registerAll(allPlanks())
	registerAll(allPotato())
	registerAll(allPrismarine())
	registerAll(allPumpkinStems())
	registerAll(allPumpkins())
	registerAll(allQuartz())
	registerAll(allQuartzStairs())
	registerAll(allSandstoneStairs())
	registerAll(allSandstones())
	registerAll(allSeaPickles())
	registerAll(allSigns())
	registerAll(allStainedGlass())
	registerAll(allStainedGlassPane())
	registerAll(allStainedTerracotta())
	registerAll(allStoneBrickStairs())
	registerAll(allStoneBricks())
	registerAll(allTallGrass())
	registerAll(allTorches())
	registerAll(allTrapdoors())
	registerAll(allWater())
	registerAll(allWheat())
	registerAll(allWood())
	registerAll(allWoodSlabs())
	registerAll(allWoodStairs())
	registerAll(allWool())
}

func init() {
	world.RegisterItem(Air{})
	world.RegisterItem(Amethyst{})
	world.RegisterItem(AncientDebris{})
	world.RegisterItem(Andesite{Polished: true})
	world.RegisterItem(Andesite{})
	world.RegisterItem(Barrel{})
	world.RegisterItem(Barrier{})
	world.RegisterItem(Basalt{Polished: true})
	world.RegisterItem(Basalt{})
	world.RegisterItem(Beacon{})
	world.RegisterItem(Bedrock{})
	world.RegisterItem(BeetrootSeeds{})
	world.RegisterItem(BlueIce{})
	world.RegisterItem(Bone{})
	world.RegisterItem(Bookshelf{})
	world.RegisterItem(Bricks{})
	world.RegisterItem(Cactus{})
	world.RegisterItem(Cake{})
	world.RegisterItem(Calcite{})
	world.RegisterItem(Carrot{})
	world.RegisterItem(Chain{})
	world.RegisterItem(Chest{})
	world.RegisterItem(ChiseledQuartz{})
	world.RegisterItem(Clay{})
	world.RegisterItem(Coal{})
	world.RegisterItem(CobblestoneStairs{Mossy: true})
	world.RegisterItem(CobblestoneStairs{})
	world.RegisterItem(Cobblestone{Mossy: true})
	world.RegisterItem(Cobblestone{})
	world.RegisterItem(CocoaBean{})
	world.RegisterItem(CraftingTable{})
	world.RegisterItem(DeadBush{})
	world.RegisterItem(Diamond{})
	world.RegisterItem(Diorite{Polished: true})
	world.RegisterItem(Diorite{})
	world.RegisterItem(DirtPath{})
	world.RegisterItem(Dirt{Coarse: true})
	world.RegisterItem(Dirt{})
	world.RegisterItem(DragonEgg{})
	world.RegisterItem(DriedKelp{})
	world.RegisterItem(Dripstone{})
	world.RegisterItem(Emerald{})
	world.RegisterItem(EndBrickStairs{})
	world.RegisterItem(EndBricks{})
	world.RegisterItem(EndStone{})
	world.RegisterItem(Farmland{})
	world.RegisterItem(GildedBlackstone{})
	world.RegisterItem(GlassPane{})
	world.RegisterItem(Glass{})
	world.RegisterItem(Glowstone{})
	world.RegisterItem(Gold{})
	world.RegisterItem(Granite{Polished: true})
	world.RegisterItem(Granite{})
	world.RegisterItem(Grass{})
	world.RegisterItem(Gravel{})
	world.RegisterItem(Honeycomb{})
	world.RegisterItem(InvisibleBedrock{})
	world.RegisterItem(IronBars{})
	world.RegisterItem(Iron{})
	world.RegisterItem(ItemFrame{Glowing: true})
	world.RegisterItem(ItemFrame{})
	world.RegisterItem(Kelp{})
	world.RegisterItem(Ladder{})
	world.RegisterItem(Lapis{})
	world.RegisterItem(LitPumpkin{})
	world.RegisterItem(MelonSeeds{})
	world.RegisterItem(Melon{})
	world.RegisterItem(MossCarpet{})
	world.RegisterItem(MudBricks{})
	world.RegisterItem(MuddyMangroveRoots{})
	world.RegisterItem(Mud{})
	world.RegisterItem(NetherBrickFence{})
	world.RegisterItem(NetherGoldOre{})
	world.RegisterItem(NetherQuartzOre{})
	world.RegisterItem(NetherSprouts{})
	world.RegisterItem(NetherWart{})
	world.RegisterItem(Netherite{})
	world.RegisterItem(Netherrack{})
	world.RegisterItem(Note{Pitch: 24})
	world.RegisterItem(Obsidian{Crying: true})
	world.RegisterItem(Obsidian{})
	world.RegisterItem(PackedIce{})
	world.RegisterItem(PackedMud{})
	world.RegisterItem(Podzol{})
	world.RegisterItem(Potato{})
	world.RegisterItem(PumpkinSeeds{})
	world.RegisterItem(Pumpkin{Carved: true})
	world.RegisterItem(Pumpkin{})
	world.RegisterItem(QuartzBricks{})
	world.RegisterItem(QuartzPillar{})
	world.RegisterItem(QuartzStairs{Smooth: true})
	world.RegisterItem(QuartzStairs{})
	world.RegisterItem(Quartz{Smooth: true})
	world.RegisterItem(Quartz{})
	world.RegisterItem(RawCopper{})
	world.RegisterItem(RawGold{})
	world.RegisterItem(RawIron{})
	world.RegisterItem(ReinforcedDeepslate{})
	world.RegisterItem(SandstoneStairs{Red: true, Smooth: true})
	world.RegisterItem(SandstoneStairs{Red: true})
	world.RegisterItem(SandstoneStairs{Smooth: true})
	world.RegisterItem(SandstoneStairs{})
	world.RegisterItem(Sand{Red: true})
	world.RegisterItem(Sand{})
	world.RegisterItem(SeaLantern{})
	world.RegisterItem(SeaPickle{})
	world.RegisterItem(Shroomlight{})
	world.RegisterItem(Snow{})
	world.RegisterItem(SoulSand{})
	world.RegisterItem(SoulSoil{})
	world.RegisterItem(Sponge{Wet: true})
	world.RegisterItem(Sponge{})
	world.RegisterItem(SporeBlossom{})
	world.RegisterItem(StoneBrickStairs{Mossy: true})
	world.RegisterItem(StoneBrickStairs{})
	world.RegisterItem(Stone{Smooth: true})
	world.RegisterItem(Stone{})
	world.RegisterItem(Terracotta{})
	world.RegisterItem(Tuff{})
	world.RegisterItem(WheatSeeds{})
	world.RegisterItem(item.Bucket{Content: Lava{}})
	world.RegisterItem(item.Bucket{Content: Water{}})

	for _, b := range allLight() {
		world.RegisterItem(b.(world.Item))
	}
	for _, c := range allCoral() {
		world.RegisterItem(c.(world.Item))
	}
	for _, c := range allCoralBlocks() {
		world.RegisterItem(c.(world.Item))
	}
	for _, s := range allSandstones() {
		world.RegisterItem(s.(world.Item))
	}
	for _, s := range allStoneBricks() {
		world.RegisterItem(s.(world.Item))
	}
	for _, c := range item.Colours() {
		world.RegisterItem(Carpet{Colour: c})
		world.RegisterItem(ConcretePowder{Colour: c})
		world.RegisterItem(Concrete{Colour: c})
		world.RegisterItem(GlazedTerracotta{Colour: c})
		world.RegisterItem(StainedGlassPane{Colour: c})
		world.RegisterItem(StainedGlass{Colour: c})
		world.RegisterItem(StainedTerracotta{Colour: c})
		world.RegisterItem(Wool{Colour: c})
	}
	for _, w := range WoodTypes() {
		if w != WarpedWood() && w != CrimsonWood() {
			world.RegisterItem(Leaves{Wood: w, Persistent: true})
		}
		world.RegisterItem(Log{Wood: w, Stripped: true})
		world.RegisterItem(Log{Wood: w})
		world.RegisterItem(Planks{Wood: w})
		world.RegisterItem(Sign{Wood: w})
		world.RegisterItem(WoodDoor{Wood: w})
		world.RegisterItem(WoodFenceGate{Wood: w})
		world.RegisterItem(WoodFence{Wood: w})
		world.RegisterItem(WoodSlab{Wood: w, Double: true})
		world.RegisterItem(WoodSlab{Wood: w})
		world.RegisterItem(WoodStairs{Wood: w})
		world.RegisterItem(WoodTrapdoor{Wood: w})
		world.RegisterItem(Wood{Wood: w, Stripped: true})
		world.RegisterItem(Wood{Wood: w})
	}
	for _, ore := range OreTypes() {
		world.RegisterItem(CoalOre{Type: ore})
		world.RegisterItem(CopperOre{Type: ore})
		world.RegisterItem(DiamondOre{Type: ore})
		world.RegisterItem(EmeraldOre{Type: ore})
		world.RegisterItem(GoldOre{Type: ore})
		world.RegisterItem(IronOre{Type: ore})
		world.RegisterItem(LapisOre{Type: ore})
	}
	for _, f := range FireTypes() {
		world.RegisterItem(Lantern{Type: f})
		world.RegisterItem(Torch{Type: f})
	}
	for _, f := range FlowerTypes() {
		world.RegisterItem(Flower{Type: f})
	}
	for _, f := range DoubleFlowerTypes() {
		world.RegisterItem(DoubleFlower{Type: f})
	}
	for _, g := range GrassTypes() {
		world.RegisterItem(DoubleTallGrass{Type: g})
		world.RegisterItem(TallGrass{Type: g})
	}
	for _, p := range PrismarineTypes() {
		world.RegisterItem(Prismarine{Type: p})
	}
	for _, t := range NetherBricksTypes() {
		world.RegisterItem(NetherBricks{Type: t})
	}
	for _, t := range FroglightTypes() {
		world.RegisterItem(Froglight{Type: t})
	}
}

func registerAll(blocks []world.Block) {
	for _, b := range blocks {
		world.RegisterBlock(b)
	}
}
