package block

// MobSpawner is a cage-like block that periodically spawns mobs when a player is nearby.
// Spawning behaviour is stored in its block entity data.
type MobSpawner struct {
	solid
	transparent
	bassDrum
}

// BreakInfo ...
func (MobSpawner) BreakInfo() BreakInfo {
	return newBreakInfo(5, pickaxeHarvestable, pickaxeEffective, simpleDrops()).withBlastResistance(25)
}

// EncodeItem ...
func (MobSpawner) EncodeItem() (name string, meta int16) {
	return "minecraft:mob_spawner", 0
}

// EncodeBlock ...
func (MobSpawner) EncodeBlock() (string, map[string]any) {
	return "minecraft:mob_spawner", nil
}

// EncodeNBT encodes the mob spawner block entity.
func (MobSpawner) EncodeNBT() map[string]any {
	return map[string]any{"id": "MobSpawner"}
}

// DecodeNBT decodes the mob spawner block entity. Spawning behaviour is not yet simulated.
func (m MobSpawner) DecodeNBT(map[string]any) any {
	return m
}
