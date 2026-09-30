package entity

import (
	"github.com/df-mc/dragonfly/server/item"
	"slices"
)

// ProjectileCharge identifies how holding an item's use changes its launch.
type ProjectileCharge uint8

const (
	ProjectileChargeNone ProjectileCharge = iota
	ProjectileChargeBow
	ProjectileChargeTrident
)

// ProjectileProfile describes a vanilla Bedrock air launch. HeightOffset is
// measured from the shooter's eyes; AngleOffset is dampened with pitch before
// launch. Bow speed is its fully drawn speed; tridents require a completed draw.
// These profiles exclude firework crossbows, Riptide, liquid drag, and spread.
type ProjectileProfile struct {
	ID, ItemType, EntityType  string
	Physics                   ProjectilePhysics
	HeightOffset, AngleOffset float64
	Charge                    ProjectileCharge
	RandomSpread              bool
}

// Vanilla values come from Mojang's Bedrock 1.26.50 entity definitions at
// https://github.com/Mojang/bedrock-samples/tree/main/behavior_pack/entities.
// A projectile component's inertia is converted to the drag fraction consumed
// by the predictor at Bedrock's single precision.
const projectileAirDrag = 1 - float64(float32(0.99))

var projectileProfiles = []ProjectileProfile{
	{ID: "pearl", ItemType: "minecraft:ender_pearl", EntityType: "minecraft:ender_pearl", Physics: ProjectilePhysics{Speed: 1.5, Gravity: 0.025}},
	{ID: "snowball", ItemType: "minecraft:snowball", EntityType: "minecraft:snowball", Physics: ProjectilePhysics{Speed: 1.5, Gravity: 0.03, Drag: projectileAirDrag}, HeightOffset: -0.1},
	{ID: "egg", ItemType: "minecraft:egg", EntityType: "minecraft:egg", Physics: ProjectilePhysics{Speed: 1.5, Gravity: 0.03, Drag: projectileAirDrag}},
	{ID: "bow", ItemType: "minecraft:bow", EntityType: "minecraft:arrow", Physics: ProjectilePhysics{Speed: 3, Gravity: 0.05, Drag: projectileAirDrag}, HeightOffset: -0.1, Charge: ProjectileChargeBow, RandomSpread: true},
	{ID: "crossbow", ItemType: "minecraft:crossbow", EntityType: "minecraft:arrow", Physics: ProjectilePhysics{Speed: 3.15, Gravity: 0.05, Drag: projectileAirDrag}, HeightOffset: -0.1, RandomSpread: true},
	{ID: "splash_potion", ItemType: "minecraft:splash_potion", EntityType: "minecraft:splash_potion", Physics: ProjectilePhysics{Speed: 0.5, Gravity: 0.05, Drag: projectileAirDrag}, AngleOffset: -20},
	{ID: "lingering_potion", ItemType: "minecraft:lingering_potion", EntityType: "minecraft:lingering_potion", Physics: ProjectilePhysics{Speed: 0.5, Gravity: 0.05, Drag: projectileAirDrag}, AngleOffset: -20},
	{ID: "xp_bottle", ItemType: "minecraft:experience_bottle", EntityType: "minecraft:xp_bottle", Physics: ProjectilePhysics{Speed: 0.5, Gravity: 0.05, Drag: projectileAirDrag}, AngleOffset: -20},
	{ID: "trident", ItemType: "minecraft:trident", EntityType: "minecraft:thrown_trident", Physics: ProjectilePhysics{Speed: 4, Gravity: 0.1, Drag: projectileAirDrag}, HeightOffset: -0.1, Charge: ProjectileChargeTrident, RandomSpread: true},
	{ID: "wind_charge", ItemType: "minecraft:wind_charge", EntityType: "minecraft:wind_charge_projectile", Physics: ProjectilePhysics{Speed: 1.5}, RandomSpread: true},
}

// ProjectileProfiles returns the supported vanilla Bedrock launch profiles.
// The returned slice is independent of the catalog and may be modified.
func ProjectileProfiles() []ProjectileProfile {
	return slices.Clone(projectileProfiles)
}

// ProjectileProfileByID finds a profile by its stable ID.
func ProjectileProfileByID(id string) (ProjectileProfile, bool) {
	for _, profile := range projectileProfiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return ProjectileProfile{}, false
}

// ProjectileProfileForItem finds the profile for an encoded held-item type.
func ProjectileProfileForItem(itemType string) (ProjectileProfile, bool) {
	for _, profile := range projectileProfiles {
		if profile.ItemType == itemType {
			return profile, true
		}
	}
	return ProjectileProfile{}, false
}

// ChargePower returns the fraction of maximum speed a held item can launch
// after drawing for ticks. Zero means the draw cannot release a projectile.
func (p ProjectileProfile) ChargePower(ticks int) float64 {
	switch p.Charge {
	case ProjectileChargeNone:
		return 1
	case ProjectileChargeBow:
		return item.BowDrawPower(ticks)
	case ProjectileChargeTrident:
		if ticks >= 10 {
			return 1
		}
	}
	return 0
}
