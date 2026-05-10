package particle

import "image/color"

// HugeExplosion is a particle shown when TNT or a creeper explodes.
type HugeExplosion struct{ particle }

// EndermanTeleport is a particle that shows up when an enderman teleports.
type EndermanTeleport struct{ particle }

// SnowballPoof is a particle shown when a snowball collides with something.
type SnowballPoof struct{ particle }

// EggSmash is a particle shown when an egg smashes on something.
type EggSmash struct{ particle }

// Splash is a particle that shows up when a splash potion is splashed.
type Splash struct {
	particle

	// Colour is the colour that should be splashed.
	Colour color.RGBA
}

// Effect is a particle that shows up around an entity when it has effects on.
type Effect struct {
	particle

	// Colour is the colour of the particle.
	Colour color.RGBA
}

// EntityFlame is a particle shown when an entity is set on fire.
type EntityFlame struct{ particle }

// Heart is a particle shown for breeding and taming animals (heart shape).
type Heart struct {
	particle
	// Scale controls the size of the heart particle.
	Scale int
}

// Critical is a particle shown when a critical hit is dealt.
type Critical struct {
	particle
	// Scale controls the size of the critical particle.
	Scale int
}

// Smoke is a particle shown for torches, furnaces, etc.
type Smoke struct {
	particle
	// Scale controls the size of the smoke particle.
	Scale int
}

// Portal is a particle shown around nether portals and endermen.
type Portal struct{ particle }

// Explode is a smaller explosion particle (not HugeExplosion).
type Explode struct{ particle }

// AngryVillager is a particle shown when a villager is angry.
type AngryVillager struct{ particle }

// HappyVillager is a particle shown when a villager is happy (green sparkles).
type HappyVillager struct{ particle }

// Bubble is a particle shown underwater.
type Bubble struct{ particle }

// Named is a particle that uses the modern Bedrock Edition string-based particle system.
// This allows spawning any particle by its identifier (e.g., "minecraft:heart_particle").
// See https://minecraft.wiki/w/Particles_(Bedrock_Edition) for available particles.
type Named struct {
	particle
	// Name is the particle identifier (e.g., "minecraft:heart_particle", "minecraft:totem_particle").
	Name string
}
