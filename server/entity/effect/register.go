package effect

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

// Register registers an effect type with its disk and network ID. Types that
// provide metadata without server-side behaviour are available through
// MetadataByID, but ByID returns only implemented effects.
func Register(id int, e MetadataType) {
	if previous, ok := effects[id].(Type); ok && effectIds[previous] == id {
		delete(effectIds, previous)
	}
	effects[id] = e
	if implementation, ok := e.(Type); ok {
		effectIds[implementation] = id
	}
}

// init registers all known effect types.
func init() {
	Register(1, Speed)
	Register(2, Slowness)
	Register(3, Haste)
	Register(4, MiningFatigue)
	Register(5, Strength)
	Register(6, InstantHealth)
	Register(7, InstantDamage)
	Register(8, JumpBoost)
	Register(9, Nausea)
	Register(10, Regeneration)
	Register(11, Resistance)
	Register(12, FireResistance)
	Register(13, WaterBreathing)
	Register(14, Invisibility)
	Register(15, Blindness)
	Register(16, NightVision)
	Register(17, Hunger)
	Register(18, Weakness)
	Register(19, Poison)
	Register(20, Wither)
	Register(21, HealthBoost)
	Register(22, Absorption)
	Register(23, Saturation)
	Register(24, Levitation)
	Register(25, FatalPoison)
	Register(26, ConduitPower)
	Register(27, SlowFalling)
	Register(30, Darkness)

	// These types describe client effects without server-side implementations.
	Register(packet.EffectBadOmen, BadOmen)
	Register(packet.EffectVillageHero, VillageHero)
	Register(packet.EffectTrialOmen, TrialOmen)
	Register(packet.EffectWindCharged, WindCharged)
	Register(packet.EffectWeaving, Weaving)
	Register(packet.EffectOozing, Oozing)
	Register(packet.EffectInfested, Infested)
	Register(packet.EffectRaidOmen, RaidOmen)
}

var (
	effects   = map[int]MetadataType{}
	effectIds = map[Type]int{}
)

// ByID returns the implementation registered with id. The bool is false for
// unknown effects and known effects without server-side behaviour.
func ByID(id int) (Type, bool) {
	effect, ok := effects[id].(Type)
	return effect, ok
}

// ID attempts to return the ID an effect was registered with. If found, the id is returned and
// the bool true.
func ID(e Type) (int, bool) {
	id, ok := effectIds[e]
	return id, ok
}
