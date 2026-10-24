package player

import (
	"sync/atomic"
	"testing"

	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/go-gl/mathgl/mgl64"
)

// ridingTickGenerator counts terrain loads caused by the player's collision work.
type ridingTickGenerator struct {
	world.NopGenerator
	generated atomic.Int64
}

// GenerateChunk records each newly visited chunk without creating blocks.
func (g *ridingTickGenerator) GenerateChunk(world.ChunkPos, *chunk.Chunk) { g.generated.Add(1) }

// ridingTickVehicle adds one seat to the existing stationary entity type.
type ridingTickVehicle struct{ world.EntityType }

// Seats places the rider at the vehicle's authoritative position.
func (ridingTickVehicle) Seats() []world.Seat { return []world.Seat{{}} }

func TestMountedPlayerTickDoesNotSweepVehicleVelocity(t *testing.T) {
	generator := &ridingTickGenerator{}
	w := world.Config{Synchronous: true, Generator: generator}.New()
	defer w.Close()
	position := mgl64.Vec3{8, 70, 8}
	vehicle := world.EntitySpawnOpts{Position: position}.New(ridingTickVehicle{entity.TextType}, entity.StationaryBehaviourConfig{})
	rider := world.EntitySpawnOpts{}.New(Type, Config{Position: position, GameMode: world.GameModeCreative})
	w.Do(func(tx *world.Tx) {
		v, p := tx.AddEntity(vehicle), tx.AddEntity(rider).(*Player)
		if !tx.Mount(p, v, 0) {
			t.Fatal("mount failed")
		}
		for _, speed := range []float64{24, 10000} {
			v.(*entity.Ent).SetVelocity(mgl64.Vec3{speed, speed, speed})
			tx.UpdateRiders(v)
			p.Tick(tx, 1)
			if generator.generated.Load() != 1 {
				t.Fatalf("mounted collision sweep visited terrain along velocity %v", p.Velocity())
			}
			if p.Velocity() != (mgl64.Vec3{}) || v.(*entity.Ent).Velocity() != (mgl64.Vec3{speed, speed, speed}) {
				t.Fatal("vehicle interpolation leaked into walking velocity")
			}
		}
	})
}

func TestDetachClearsVehicleInterpolationBeforeWalking(t *testing.T) {
	for _, remove := range []bool{false, true} {
		generator := &ridingTickGenerator{}
		w := world.Config{Synchronous: true, Generator: generator}.New()
		position := mgl64.Vec3{8, 70, 8}
		vehicle := world.EntitySpawnOpts{Position: position}.New(ridingTickVehicle{entity.TextType}, entity.StationaryBehaviourConfig{})
		rider := world.EntitySpawnOpts{}.New(Type, Config{Position: position, GameMode: world.GameModeCreative})
		w.Do(func(tx *world.Tx) {
			v, p := tx.AddEntity(vehicle), tx.AddEntity(rider).(*Player)
			if !tx.Mount(p, v, 0) {
				t.Fatal("mount failed")
			}
			v.(*entity.Ent).SetVelocity(mgl64.Vec3{10000, 10000, 10000})
			tx.UpdateRiders(v)
			if remove {
				tx.RemoveEntity(v)
			} else if !tx.Dismount(p) {
				t.Fatal("dismount failed")
			}
			if p.Velocity() != (mgl64.Vec3{}) {
				t.Fatal("detached player retained vehicle interpolation velocity")
			}
			p.Tick(tx, 2)
			if generator.generated.Load() != 1 {
				t.Fatal("detached player swept terrain along vehicle velocity")
			}
		})
		w.Close()
	}
}
