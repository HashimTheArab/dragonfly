package effect

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

// Register registers an effect's metadata and optional implementation for disk
// and network translation. A nil implementation describes a client effect that
// has no server-side behaviour here; ByID will not return it as implemented.
func Register(id int, e Type, metadata Metadata) {
	if previous, ok := effects[id]; ok && previous.implementation != nil && effectIds[previous.implementation] == id {
		delete(effectIds, previous.implementation)
	}
	effects[id] = registeredEffect{implementation: e, metadata: metadata}
	if e != nil {
		effectIds[e] = id
	}
}

// init registers all known effects, with implementations where available.
func init() {
	Register(packet.EffectSpeed, Speed, Metadata{"speed", "speed_effect"})
	Register(packet.EffectSlowness, Slowness, Metadata{"slowness", "slowness_effect"})
	Register(packet.EffectHaste, Haste, Metadata{"haste", "haste_effect"})
	Register(packet.EffectMiningFatigue, MiningFatigue, Metadata{"mining_fatigue", "mining_fatigue_effect"})
	Register(packet.EffectStrength, Strength, Metadata{"strength", "strength_effect"})
	Register(packet.EffectInstantHealth, InstantHealth, Metadata{"instant_health", "health_boost_effect"})
	Register(packet.EffectInstantDamage, InstantDamage, Metadata{"instant_damage", "wither_effect"})
	Register(packet.EffectJumpBoost, JumpBoost, Metadata{"jump_boost", "jump_boost_effect"})
	Register(packet.EffectNausea, Nausea, Metadata{"nausea", "nausea_effect"})
	Register(packet.EffectRegeneration, Regeneration, Metadata{"regeneration", "regeneration_effect"})
	Register(packet.EffectResistance, Resistance, Metadata{"resistance", "resistance_effect"})
	Register(packet.EffectFireResistance, FireResistance, Metadata{"fire_resistance", "fire_resistance_effect"})
	Register(packet.EffectWaterBreathing, WaterBreathing, Metadata{"water_breathing", "water_breathing_effect"})
	Register(packet.EffectInvisibility, Invisibility, Metadata{"invisibility", "invisibility_effect"})
	Register(packet.EffectBlindness, Blindness, Metadata{"blindness", "blindness_effect"})
	Register(packet.EffectNightVision, NightVision, Metadata{"night_vision", "night_vision_effect"})
	Register(packet.EffectHunger, Hunger, Metadata{"hunger", "hunger_effect"})
	Register(packet.EffectWeakness, Weakness, Metadata{"weakness", "weakness_effect"})
	Register(packet.EffectPoison, Poison, Metadata{"poison", "poison_effect"})
	Register(packet.EffectWither, Wither, Metadata{"wither", "wither_effect"})
	Register(packet.EffectHealthBoost, HealthBoost, Metadata{"health_boost", "health_boost_effect"})
	Register(packet.EffectAbsorption, Absorption, Metadata{"absorption", "absorption_effect"})
	Register(packet.EffectSaturation, Saturation, Metadata{"saturation", "hunger_effect"})
	Register(packet.EffectLevitation, Levitation, Metadata{"levitation", "levitation_effect"})
	Register(packet.EffectFatalPoison, FatalPoison, Metadata{"fatal_poison", "poison_effect"})
	Register(packet.EffectConduitPower, ConduitPower, Metadata{"conduit_power", "conduit_power_effect"})
	Register(packet.EffectSlowFalling, SlowFalling, Metadata{"slow_falling", "slow_falling_effect"})
	Register(packet.EffectBadOmen, nil, Metadata{"bad_omen", "bad_omen_effect"})
	Register(packet.EffectVillageHero, nil, Metadata{"village_hero", "village_hero_effect"})
	Register(packet.EffectDarkness, Darkness, Metadata{"darkness", "darkness_effect"})
	Register(packet.EffectTrialOmen, nil, Metadata{"trial_omen", "trial_omen_effect"})
	Register(packet.EffectWindCharged, nil, Metadata{"wind_charged", "wind_charged_effect"})
	Register(packet.EffectWeaving, nil, Metadata{"weaving", "weaving_effect"})
	Register(packet.EffectOozing, nil, Metadata{"oozing", "oozing_effect"})
	Register(packet.EffectInfested, nil, Metadata{"infested", "infested_effect"})
	Register(packet.EffectRaidOmen, nil, Metadata{"raid_omen", "raid_omen_effect"})
}

// registeredEffect keeps presentation and implementation under one registry owner.
type registeredEffect struct {
	implementation Type
	metadata       Metadata
}

var (
	effects   = map[int]registeredEffect{}
	effectIds = map[Type]int{}
)

// ByID returns the implementation registered with id. The bool is false for
// unknown effects and known effects without server-side behaviour.
func ByID(id int) (Type, bool) {
	effect, ok := effects[id]
	return effect.implementation, ok && effect.implementation != nil
}

// ID attempts to return the ID an effect was registered with. If found, the id is returned and
// the bool true.
func ID(e Type) (int, bool) {
	id, ok := effectIds[e]
	return id, ok
}
