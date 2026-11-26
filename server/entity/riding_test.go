package entity

import (
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// ridingVehicleType gives a stationary test actor one passenger seat.
type ridingVehicleType struct{ world.EntityType }

// Seats places the passenger above its vehicle.
func (ridingVehicleType) Seats() []world.Seat {
	return []world.Seat{{Position: mgl64.Vec3{0, 1, 0}}}
}

func TestMountedEntityPhysicsPreservesSeat(t *testing.T) {
	for name, config := range map[string]world.EntityConfig{
		"passive":    PassiveBehaviourConfig{Gravity: 0.08, Drag: 0.02},
		"projectile": ProjectileBehaviourConfig{Gravity: 0.05, Drag: 0.01},
		"firework":   FireworkBehaviourConfig{ExistenceDuration: time.Minute, UpwardsAcceleration: 0.04},
	} {
		t.Run(name, func(t *testing.T) {
			for _, riderFirst := range []bool{false, true} {
				w := world.Config{Synchronous: true}.New()
				vehicle := world.EntitySpawnOpts{Position: mgl64.Vec3{8, 70, 8}}.New(ridingVehicleType{TextType}, StationaryBehaviourConfig{})
				rider := world.EntitySpawnOpts{}.New(TextType, config)
				w.Do(func(tx *world.Tx) {
					v, r := tx.AddEntity(vehicle).(*Ent), tx.AddEntity(rider).(*Ent)
					if !tx.Mount(r, v, 0) {
						t.Fatal("mount failed")
					}
					if riderFirst {
						r.Tick(tx, 1)
						if r.Position() != (mgl64.Vec3{8, 71, 8}) || r.Velocity() != (mgl64.Vec3{}) {
							t.Fatal("rider moved before the vehicle tick")
						}
					}
					v.Teleport(mgl64.Vec3{10, 72, 10})
					tx.UpdateRiders(v)
					if !riderFirst {
						r.Tick(tx, 1)
					}
					if r.Position() != (mgl64.Vec3{10, 73, 10}) || r.Velocity() != (mgl64.Vec3{}) {
						t.Fatalf("independent rider physics changed seat: %v, %v", r.Position(), r.Velocity())
					}
					if r.Age() != time.Second/20 {
						t.Fatal("mounted entity stopped aging")
					}
					if !tx.Dismount(r) {
						t.Fatal("dismount failed")
					}
					r.Tick(tx, 2)
					if r.Position() == (mgl64.Vec3{10, 73, 10}) && r.Velocity() == (mgl64.Vec3{}) {
						t.Fatal("dismounted entity did not resume physics")
					}
				})
				w.Close()
			}
		})
	}
}

func TestMountedPassiveEntityStillExpires(t *testing.T) {
	w := world.Config{Synchronous: true}.New()
	defer w.Close()
	expired := false
	vehicle := world.EntitySpawnOpts{Position: mgl64.Vec3{8, 70, 8}}.New(ridingVehicleType{TextType}, StationaryBehaviourConfig{})
	rider := world.EntitySpawnOpts{}.New(TextType, PassiveBehaviourConfig{
		Gravity:           0.08,
		ExistenceDuration: time.Second / 20,
		Expire:            func(*Ent, *world.Tx) { expired = true },
	})
	w.Do(func(tx *world.Tx) {
		v, r := tx.AddEntity(vehicle), tx.AddEntity(rider).(*Ent)
		if !tx.Mount(r, v, 0) {
			t.Fatal("mount failed")
		}
		for tick := int64(1); tick <= 4; tick++ {
			r.Tick(tx, tick)
		}
		if !expired || !rider.Closed() || len(world.RiderLinks(v)) != 0 {
			t.Fatal("mounted expiry did not close the actor and release its seat")
		}
	})
}
