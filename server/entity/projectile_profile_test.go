package entity

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

func TestProjectileProfiles_CatalogLookupsAndIsolation(t *testing.T) {
	ids, items := map[string]bool{}, map[string]bool{}
	for _, profile := range ProjectileProfiles() {
		if profile.ID == "" || profile.ItemType == "" || profile.EntityType == "" || ids[profile.ID] || items[profile.ItemType] {
			t.Fatalf("empty or repeated catalog identity: %+v", profile)
		}
		ids[profile.ID], items[profile.ItemType] = true, true
		byID, idFound := ProjectileProfileByID(profile.ID)
		byItem, itemFound := ProjectileProfileForItem(profile.ItemType)
		if !idFound || !itemFound || byID != profile || byItem != profile {
			t.Fatalf("catalog lookup diverged: %+v", profile)
		}
		if profile.Physics.Speed <= 0 || profile.Physics.Gravity < 0 || profile.Physics.Drag < 0 || profile.Physics.Drag >= 1 {
			t.Fatalf("invalid profile physics: %+v", profile)
		}
	}
	copy := ProjectileProfiles()
	copy[0].ID = "changed"
	if _, ok := ProjectileProfileByID("changed"); ok {
		t.Fatal("caller mutated shared catalog")
	}
	if _, ok := ProjectileProfileByID("missing"); ok {
		t.Fatal("unknown ID found")
	}
	if _, ok := ProjectileProfileForItem("minecraft:stone"); ok {
		t.Fatal("non-projectile item found")
	}
}

func TestProjectileProfiles_VanillaLaunchKinds(t *testing.T) {
	pearl, _ := ProjectileProfileByID("pearl")
	if pearl.Physics != (ProjectilePhysics{Speed: 1.5, Gravity: .025}) || pearl.HeightOffset != 0 || pearl.AngleOffset != 0 {
		t.Fatalf("BDS pearl profile = %+v", pearl)
	}
	bow, _ := ProjectileProfileByID("bow")
	crossbow, _ := ProjectileProfileByID("crossbow")
	if bow.Physics.Speed != 3 || crossbow.Physics.Speed != 3.15 || bow.Charge != ProjectileChargeBow || crossbow.Charge != ProjectileChargeNone {
		t.Fatalf("charged arrow profiles = %+v, %+v", bow, crossbow)
	}
	trident, _ := ProjectileProfileByID("trident")
	if trident.EntityType != "minecraft:thrown_trident" || trident.Charge != ProjectileChargeTrident {
		t.Fatalf("trident identity/draw = %+v", trident)
	}
	potion, _ := ProjectileProfileByID("splash_potion")
	if potion.AngleOffset != -20 || potion.Physics.Speed != .5 {
		t.Fatalf("potion angled launch = %+v", potion)
	}
	wind, _ := ProjectileProfileByID("wind_charge")
	if wind.Physics.Gravity != 0 || wind.Physics.Drag != 0 || wind.EntityType != "minecraft:wind_charge_projectile" {
		t.Fatalf("wind charge profile = %+v", wind)
	}
}

func TestProjectileLaunchVelocity_AngleOffsetDampensAtVerticalAim(t *testing.T) {
	for _, pitch := range []float64{-90, 90} {
		rotation := cube.Rotation{30, pitch}
		withOffset := ProjectileLaunchVelocity(rotation, .5, -20)
		withoutOffset := ProjectileLaunchVelocity(rotation, .5, 0)
		if withOffset != withoutOffset {
			t.Fatalf("vertical aim %g changed by offset: %v instead of %v", pitch, withOffset, withoutOffset)
		}
	}
	flat := ProjectileLaunchVelocity(cube.Rotation{}, .5, -20)
	if flat[1] <= 0 || flat.Sub(mgl64.Vec3{0, .171, .47}).Len() > .001 {
		t.Fatalf("flat potion aim = %v", flat)
	}
}

func TestProjectileLaunchVelocity_ObservedBDSAngleOffsets(t *testing.T) {
	// Official BDS 1.26.50.5 emitted these identical potion and XP-bottle
	// AddActor velocities at yaw 30. Values are widened from packet float32.
	for _, shot := range []struct {
		pitch    float64
		velocity mgl64.Vec3
	}{
		{0, mgl64.Vec3{-.23491739, .17097005, .40691903}},
		{45, mgl64.Vec3{-.21460159, -.25644514, .37172845}},
		{-45, mgl64.Vec3{-.12823062, .429209, .22211842}},
		{30, mgl64.Vec3{-.24389072, -.1097399, .42246243}},
		{-30, mgl64.Vec3{-.16947237, .36756453, .2935565}},
	} {
		for axis := range shot.velocity {
			shot.velocity[axis] = float64(float32(shot.velocity[axis]))
		}
		got := ProjectileLaunchVelocity(cube.Rotation{30, shot.pitch}, .5, -20)
		// Bedrock's native sine table and Go's sine evaluation may differ by
		// one final float32 ULP. Both must preserve the dampened launch angle.
		if got.Sub(shot.velocity).Len() > 3e-8 {
			t.Errorf("pitch %g launch = %v; observed %v", shot.pitch, got, shot.velocity)
		}
	}
}

func TestProjectileProfiles_ChargePower(t *testing.T) {
	bow, _ := ProjectileProfileByID("bow")
	trident, _ := ProjectileProfileByID("trident")
	pearl, _ := ProjectileProfileByID("pearl")
	if bow.ChargePower(2) != 0 || bow.ChargePower(20) != 1 || bow.ChargePower(10) <= 0 || bow.ChargePower(10) >= 1 {
		t.Fatal("bow charge threshold or scaling changed")
	}
	// Official BDS releases at 450ms emitted no entity, while 500ms did.
	if trident.ChargePower(9) != 0 || trident.ChargePower(10) != 1 {
		t.Fatal("trident release threshold changed")
	}
	if pearl.ChargePower(0) != 1 {
		t.Fatal("uncharged projectile requires a draw")
	}
}
