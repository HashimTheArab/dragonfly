# Missing obtainable blocks checklist

This checklist tracks block IDs that are still present as unknown states in the
local Bedrock palette after the olebeck block port, filtered for blocks that are
useful as normal creative/survival building blocks.

Source baseline:

- Local palette scan: `server/world/block_states.nbt`
- External reference: <https://minecraft.wiki/w/Block>

Initial scan after the olebeck port:

- Unknown block names remaining: **486**
- Unknown block states remaining: **7,984**

Shell-only pass status: **completed**. The checked block IDs are now registered
through `server/block/shell_building_block.go` where they did not already have a
native implementation.

Scan after the shell-only pass:

- Unknown block names remaining: **197**
- Unknown block states remaining: **529**

Remaining unknown palette states are intentionally excluded technical/Education/
deprecated/internal states or special existing block states such as item-frame
map/photo variants.

Many of those are technical, Education Edition, deprecated, or internal blocks,
so they are intentionally separated from the building-focused target list below.

## Implementation policy for this pass

Goal: make blocks exist for building.

Prefer **shell-only implementations**:

- encode/register block states and item IDs;
- simple model/collision where obvious, otherwise reuse `solid`, `transparent`,
  `empty`, `sourceWaterDisplacer`, or existing slab/stair/wall/button-like
  models;
- simple drops: usually `oneOf(block)` or existing silk-touch/shears conventions
  where trivial;
- add creative/survival item registration where the block has a normal item form;
- add map `Color()` values only when easy.

Known limitation: shell-only blocks currently return a neutral grey map colour,
so maps render these placeholders uniformly until native block implementations
or per-block colours are added.

Do **not** implement complex behaviour yet:

- no real redstone simulation;
- no piston movement;
- no inventories/block entities unless needed to avoid crashes;
- no bee storage, sculk spreading, trial/vault rewards, crafter logic, etc.;
- no full plant growth logic.

## Priority 0: fix partial registrations/encoding mismatches

These have existing Dragonfly concepts or nearby implementations, but the local
palette still has unknown names/states. Fixing them should reduce unknowns
without adding much behaviour.

- [x] `minecraft:bed`
- [x] `minecraft:bone_block`
- [x] `minecraft:hay_block`
- [x] `minecraft:grindstone`
- [x] `minecraft:ladder`
- [x] `minecraft:lectern`
- [x] `minecraft:purpur_block`
- [x] `minecraft:quartz_block`
- [x] `minecraft:smooth_quartz`
- [x] `minecraft:chiseled_quartz_block`
- [x] `minecraft:stonecutter`
- [x] `minecraft:tnt`

## Priority 1: easy shell-only building blocks

Mostly static blocks or simple non-full-cube decorative blocks.

### Amethyst

- [x] `minecraft:budding_amethyst`
- [x] `minecraft:small_amethyst_bud`
- [x] `minecraft:medium_amethyst_bud`
- [x] `minecraft:large_amethyst_bud`
- [x] `minecraft:amethyst_cluster`

### Basic/static natural blocks

- [x] `minecraft:ice`
- [x] `minecraft:frosted_ice`
- [x] `minecraft:magma`
- [x] `minecraft:honey_block`
- [x] `minecraft:powder_snow`
- [x] `minecraft:mycelium`
- [x] `minecraft:glow_lichen`
- [x] `minecraft:tinted_glass`
- [x] `minecraft:web`
- [x] `minecraft:heavy_core`
- [x] `minecraft:suspicious_gravel`
- [x] `minecraft:suspicious_sand`

### Pale garden / resin / new decorative

- [x] `minecraft:pale_moss_block`
- [x] `minecraft:pale_moss_carpet`
- [x] `minecraft:pale_hanging_moss`
- [x] `minecraft:resin_clump`
- [x] `minecraft:creaking_heart`
- [x] `minecraft:closed_eyeblossom`
- [x] `minecraft:open_eyeblossom`
- [x] `minecraft:leaf_litter`
- [x] `minecraft:wildflowers`
- [x] `minecraft:firefly_bush`
- [x] `minecraft:cactus_flower`
- [x] `minecraft:bush`
- [x] `minecraft:short_dry_grass`
- [x] `minecraft:tall_dry_grass`
- [x] `minecraft:golden_dandelion`

### Nether vegetation

- [x] `minecraft:crimson_fungus`
- [x] `minecraft:warped_fungus`
- [x] `minecraft:crimson_roots`
- [x] `minecraft:warped_roots`
- [x] `minecraft:crimson_nylium`
- [x] `minecraft:warped_nylium`
- [x] `minecraft:twisting_vines`
- [x] `minecraft:weeping_vines`

### Lush/crop decorative shells

- [x] `minecraft:big_dripleaf`
- [x] `minecraft:small_dripleaf_block`
- [x] `minecraft:cave_vines`
- [x] `minecraft:cave_vines_body_with_berries`
- [x] `minecraft:cave_vines_head_with_berries`
- [x] `minecraft:torchflower`
- [x] `minecraft:torchflower_crop`
- [x] `minecraft:pitcher_plant`
- [x] `minecraft:pitcher_crop`
- [x] `minecraft:sweet_berry_bush`

### Eggs/spawn-like building blocks

- [x] `minecraft:turtle_egg`
- [x] `minecraft:sniffer_egg`
- [x] `minecraft:frog_spawn`

### Additional natural/building shells

- [x] `minecraft:chorus_flower`
- [x] `minecraft:chorus_plant`
- [x] `minecraft:dried_ghast`
- [x] `minecraft:mangrove_propagule`
- [x] `minecraft:mangrove_roots`
- [x] `minecraft:pointed_dripstone`
- [x] `minecraft:scaffolding`

### Infested blocks

- [x] `minecraft:infested_stone`
- [x] `minecraft:infested_cobblestone`
- [x] `minecraft:infested_stone_bricks`
- [x] `minecraft:infested_mossy_stone_bricks`
- [x] `minecraft:infested_cracked_stone_bricks`
- [x] `minecraft:infested_chiseled_stone_bricks`
- [x] `minecraft:infested_deepslate`

## Priority 2: wood/variant families

These are good shell-only targets, but they fan out into many states/variants.
Prefer shared generic types rather than one file per wood type.

### Bamboo family

- [x] `minecraft:bamboo`
- [x] `minecraft:bamboo_sapling`
- [x] `minecraft:bamboo_block`
- [x] `minecraft:stripped_bamboo_block`
- [x] `minecraft:bamboo_planks`
- [x] `minecraft:bamboo_mosaic`
- [x] `minecraft:bamboo_slab`
- [x] `minecraft:bamboo_double_slab`
- [x] `minecraft:bamboo_stairs`
- [x] `minecraft:bamboo_mosaic_slab`
- [x] `minecraft:bamboo_mosaic_double_slab`
- [x] `minecraft:bamboo_mosaic_stairs`
- [x] `minecraft:bamboo_fence`
- [x] `minecraft:bamboo_fence_gate`
- [x] `minecraft:bamboo_door`
- [x] `minecraft:bamboo_trapdoor`
- [x] `minecraft:bamboo_button`
- [x] `minecraft:bamboo_pressure_plate`
- [x] `minecraft:bamboo_standing_sign`
- [x] `minecraft:bamboo_wall_sign`

### Glazed terracotta missing palette states

Existing glazed terracotta support covers normal placement. These shell states
only fill remaining palette gaps.

- [x] `minecraft:white_glazed_terracotta`
- [x] `minecraft:orange_glazed_terracotta`
- [x] `minecraft:magenta_glazed_terracotta`
- [x] `minecraft:light_blue_glazed_terracotta`
- [x] `minecraft:yellow_glazed_terracotta`
- [x] `minecraft:lime_glazed_terracotta`
- [x] `minecraft:pink_glazed_terracotta`
- [x] `minecraft:gray_glazed_terracotta`
- [x] `minecraft:silver_glazed_terracotta`
- [x] `minecraft:cyan_glazed_terracotta`
- [x] `minecraft:purple_glazed_terracotta`
- [x] `minecraft:blue_glazed_terracotta`
- [x] `minecraft:brown_glazed_terracotta`
- [x] `minecraft:green_glazed_terracotta`
- [x] `minecraft:red_glazed_terracotta`
- [x] `minecraft:black_glazed_terracotta`
- [x] `minecraft:bamboo_hanging_sign`

### Buttons

- [x] `minecraft:wooden_button`
- [x] `minecraft:stone_button`
- [x] `minecraft:polished_blackstone_button`
- [x] `minecraft:oak_button`
- [x] `minecraft:spruce_button`
- [x] `minecraft:birch_button`
- [x] `minecraft:jungle_button`
- [x] `minecraft:acacia_button`
- [x] `minecraft:dark_oak_button`
- [x] `minecraft:mangrove_button`
- [x] `minecraft:cherry_button`
- [x] `minecraft:pale_oak_button`
- [x] `minecraft:crimson_button`
- [x] `minecraft:warped_button`

### Pressure plates

- [x] `minecraft:wooden_pressure_plate`
- [x] `minecraft:stone_pressure_plate`
- [x] `minecraft:polished_blackstone_pressure_plate`
- [x] `minecraft:light_weighted_pressure_plate`
- [x] `minecraft:heavy_weighted_pressure_plate`
- [x] `minecraft:oak_pressure_plate`
- [x] `minecraft:spruce_pressure_plate`
- [x] `minecraft:birch_pressure_plate`
- [x] `minecraft:jungle_pressure_plate`
- [x] `minecraft:acacia_pressure_plate`
- [x] `minecraft:dark_oak_pressure_plate`
- [x] `minecraft:mangrove_pressure_plate`
- [x] `minecraft:cherry_pressure_plate`
- [x] `minecraft:pale_oak_pressure_plate`
- [x] `minecraft:crimson_pressure_plate`
- [x] `minecraft:warped_pressure_plate`

### Hanging signs / wall sign aliases

Existing sign support should be reused where possible.

- [x] `minecraft:oak_hanging_sign`
- [x] `minecraft:spruce_hanging_sign`
- [x] `minecraft:birch_hanging_sign`
- [x] `minecraft:jungle_hanging_sign`
- [x] `minecraft:acacia_hanging_sign`
- [x] `minecraft:dark_oak_hanging_sign`
- [x] `minecraft:mangrove_hanging_sign`
- [x] `minecraft:cherry_hanging_sign`
- [x] `minecraft:pale_oak_hanging_sign`
- [x] `minecraft:crimson_hanging_sign`
- [x] `minecraft:warped_hanging_sign`
- [x] `minecraft:bamboo_hanging_sign`
- [x] `minecraft:wall_sign`
- [x] `minecraft:oak_wall_sign`
- [x] `minecraft:spruce_wall_sign`
- [x] `minecraft:birch_wall_sign`
- [x] `minecraft:jungle_wall_sign`
- [x] `minecraft:acacia_wall_sign`
- [x] `minecraft:darkoak_wall_sign`
- [x] `minecraft:mangrove_wall_sign`
- [x] `minecraft:cherry_wall_sign`
- [x] `minecraft:pale_oak_wall_sign`
- [x] `minecraft:crimson_wall_sign`
- [x] `minecraft:warped_wall_sign`
- [x] `minecraft:bamboo_wall_sign`

### Missing wooden/iron doors and trapdoors

Some door/trapdoor families already exist, but these palette names/states are
still unknown.

- [x] `minecraft:iron_door`
- [x] `minecraft:iron_trapdoor`

## Priority 3: shell-only utility/storage blocks

Implement as placeable decorative blocks first. Add inventories/UI later only if
needed.

- [x] `minecraft:cartography_table`
- [x] `minecraft:cauldron`
- [x] `minecraft:flower_pot`
- [x] `minecraft:bee_nest`
- [x] `minecraft:beehive`
- [x] `minecraft:bell`
- [x] `minecraft:chiseled_bookshelf`
- [x] `minecraft:conduit`
- [x] `minecraft:crafter`
- [x] `minecraft:lodestone`
- [x] `minecraft:respawn_anchor`
- [x] `minecraft:mob_spawner`
- [x] `minecraft:trial_spawner`
- [x] `minecraft:vault`
- [x] `minecraft:end_portal_frame`
- [x] `minecraft:trapped_chest`

### Shulker boxes

- [x] `minecraft:undyed_shulker_box`
- [x] `minecraft:white_shulker_box`
- [x] `minecraft:orange_shulker_box`
- [x] `minecraft:magenta_shulker_box`
- [x] `minecraft:light_blue_shulker_box`
- [x] `minecraft:yellow_shulker_box`
- [x] `minecraft:lime_shulker_box`
- [x] `minecraft:pink_shulker_box`
- [x] `minecraft:gray_shulker_box`
- [x] `minecraft:light_gray_shulker_box`
- [x] `minecraft:cyan_shulker_box`
- [x] `minecraft:purple_shulker_box`
- [x] `minecraft:blue_shulker_box`
- [x] `minecraft:brown_shulker_box`
- [x] `minecraft:green_shulker_box`
- [x] `minecraft:red_shulker_box`
- [x] `minecraft:black_shulker_box`

### Candle cakes

Placeable decorative shells are enough for building. They can be wired to cake
interaction later.

- [x] `minecraft:candle_cake`
- [x] `minecraft:white_candle_cake`
- [x] `minecraft:orange_candle_cake`
- [x] `minecraft:magenta_candle_cake`
- [x] `minecraft:light_blue_candle_cake`
- [x] `minecraft:yellow_candle_cake`
- [x] `minecraft:lime_candle_cake`
- [x] `minecraft:pink_candle_cake`
- [x] `minecraft:gray_candle_cake`
- [x] `minecraft:light_gray_candle_cake`
- [x] `minecraft:cyan_candle_cake`
- [x] `minecraft:purple_candle_cake`
- [x] `minecraft:blue_candle_cake`
- [x] `minecraft:brown_candle_cake`
- [x] `minecraft:green_candle_cake`
- [x] `minecraft:red_candle_cake`
- [x] `minecraft:black_candle_cake`

## Priority 4: redstone/mechanical as decorative shells

These should initially be inert placeable blocks. Functional redstone can be a
separate project.

### Rails

- [x] `minecraft:rail`
- [x] `minecraft:golden_rail`
- [x] `minecraft:detector_rail`
- [x] `minecraft:activator_rail`

### Redstone blocks/components

- [x] `minecraft:redstone_block`
- [x] `minecraft:redstone_ore`
- [x] `minecraft:lit_redstone_ore`
- [x] `minecraft:deepslate_redstone_ore`
- [x] `minecraft:lit_deepslate_redstone_ore`
- [x] `minecraft:redstone_lamp`
- [x] `minecraft:lit_redstone_lamp`
- [x] `minecraft:redstone_torch`
- [x] `minecraft:unlit_redstone_torch`
- [x] `minecraft:redstone_wire`
- [x] `minecraft:unpowered_repeater`
- [x] `minecraft:powered_repeater`
- [x] `minecraft:unpowered_comparator`
- [x] `minecraft:powered_comparator`
- [x] `minecraft:daylight_detector`
- [x] `minecraft:daylight_detector_inverted`
- [x] `minecraft:target`
- [x] `minecraft:lever`
- [x] `minecraft:tripwire_hook`
- [x] `minecraft:trip_wire`

### Pistons/dispensers/observers

- [x] `minecraft:piston`
- [x] `minecraft:sticky_piston`
- [x] `minecraft:dispenser`
- [x] `minecraft:dropper`
- [x] `minecraft:observer`

## Priority 5: copper/new metal families

These can be shell-only, but should share oxidation/waxed helpers with existing
copper code.

### Copper bulbs

- [x] `minecraft:copper_bulb`
- [x] `minecraft:exposed_copper_bulb`
- [x] `minecraft:weathered_copper_bulb`
- [x] `minecraft:oxidized_copper_bulb`
- [x] `minecraft:waxed_copper_bulb`
- [x] `minecraft:waxed_exposed_copper_bulb`
- [x] `minecraft:waxed_weathered_copper_bulb`
- [x] `minecraft:waxed_oxidized_copper_bulb`

### Lightning rods

- [x] `minecraft:lightning_rod`
- [x] `minecraft:exposed_lightning_rod`
- [x] `minecraft:weathered_lightning_rod`
- [x] `minecraft:oxidized_lightning_rod`
- [x] `minecraft:waxed_lightning_rod`
- [x] `minecraft:waxed_exposed_lightning_rod`
- [x] `minecraft:waxed_weathered_lightning_rod`
- [x] `minecraft:waxed_oxidized_lightning_rod`

### Copper chests/shelves

These are palette-present in this checkout. Treat as shell-only until the target
Bedrock version and intended gameplay are confirmed.

- [x] `minecraft:copper_chest`
- [x] `minecraft:exposed_copper_chest`
- [x] `minecraft:weathered_copper_chest`
- [x] `minecraft:oxidized_copper_chest`
- [x] `minecraft:waxed_copper_chest`
- [x] `minecraft:waxed_exposed_copper_chest`
- [x] `minecraft:waxed_weathered_copper_chest`
- [x] `minecraft:waxed_oxidized_copper_chest`
- [x] `minecraft:oak_shelf`
- [x] `minecraft:spruce_shelf`
- [x] `minecraft:birch_shelf`
- [x] `minecraft:jungle_shelf`
- [x] `minecraft:acacia_shelf`
- [x] `minecraft:dark_oak_shelf`
- [x] `minecraft:mangrove_shelf`
- [x] `minecraft:cherry_shelf`
- [x] `minecraft:pale_oak_shelf`
- [x] `minecraft:crimson_shelf`
- [x] `minecraft:warped_shelf`
- [x] `minecraft:bamboo_shelf`

## Priority 6: sculk family as decorative shells

No spreading/sensing/shrieking behaviour in the shell-only pass.

- [x] `minecraft:sculk`
- [x] `minecraft:sculk_catalyst`
- [x] `minecraft:sculk_sensor`
- [x] `minecraft:calibrated_sculk_sensor`
- [x] `minecraft:sculk_shrieker`
- [x] `minecraft:sculk_vein`

## Excluded by default

Do not add these in the building shell-only pass unless explicitly requested.

### Technical/internal/runtime blocks

- `minecraft:air` variants beyond existing air handling
- `minecraft:moving_block`
- `minecraft:piston_arm_collision`
- `minecraft:sticky_piston_arm_collision`
- `minecraft:client_request_placeholder_block`
- `minecraft:info_update`
- `minecraft:info_update2`
- `minecraft:reserved6`
- `minecraft:unknown`
- `minecraft:structure_void`
- `minecraft:frame`
- `minecraft:glow_frame`
- `minecraft:bubble_column`
- `minecraft:portal`
- `minecraft:end_portal`
- `minecraft:end_gateway`

### Command/structure/admin blocks

- `minecraft:command_block`
- `minecraft:chain_command_block`
- `minecraft:repeating_command_block`
- `minecraft:jigsaw`
- `minecraft:structure_block`

### Education Edition / chemistry blocks

- `minecraft:allow`
- `minecraft:deny`
- `minecraft:border_block`
- `minecraft:camera`
- `minecraft:chalkboard`
- `minecraft:chemical_heat`
- `minecraft:compound_creator`
- `minecraft:element_constructor`
- `minecraft:lab_table`
- `minecraft:material_reducer`
- `minecraft:element_0` through `minecraft:element_118`
- `minecraft:hard_glass`
- `minecraft:hard_glass_pane`
- `minecraft:hard_*_stained_glass`
- `minecraft:hard_*_stained_glass_pane`
- `minecraft:colored_torch_blue`
- `minecraft:colored_torch_green`
- `minecraft:colored_torch_purple`
- `minecraft:colored_torch_red`
- `minecraft:underwater_tnt`
- `minecraft:underwater_torch`

### Removed/deprecated/legacy aliases

- `minecraft:deprecated_anvil`
- `minecraft:deprecated_purpur_block_1`
- `minecraft:deprecated_purpur_block_2`
- `minecraft:glowingobsidian`
- `minecraft:netherreactor`
- `minecraft:petrified_oak_slab`
- `minecraft:petrified_oak_double_slab`

## Suggested implementation order

1. Priority 0 encoding/registration fixes.
2. Priority 1 static/decorative/natural shells.
3. Priority 2 reusable variant families.
4. Priority 3 storage/utility shells.
5. Priority 4 redstone/mechanical shells.
6. Priority 5 copper/new metal families.
7. Priority 6 sculk shells.

After each batch:

```powershell
go build ./server/...
go vet ./server/...
go test ./server/block/... ./server/world/...
go test ./...
```


