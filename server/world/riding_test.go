package world

import (
	"math"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

// ridingTestType opens an entity with observable rider callbacks.
type ridingTestType struct {
	testEntityType
	seats   []Seat
	inputs  *[]RiderInput
	changes *[]bool
}

func (t ridingTestType) Seats() []Seat { return t.seats }
func (t ridingTestType) Open(_ *Tx, h *EntityHandle, d *EntityData) Entity {
	return &ridingTestEntity{testEntity: testEntity{handle: h, data: d}, inputs: t.inputs, changes: t.changes}
}

// ridingTestEntity records input and membership changes without an asynchronous tick loop.
type ridingTestEntity struct {
	testEntity
	inputs  *[]RiderInput
	changes *[]bool
}

func (e *ridingTestEntity) HandleRiderInput(_ Entity, _ int, input RiderInput) {
	*e.inputs = append(*e.inputs, input)
}
func (e *ridingTestEntity) HandleRiderChange(_ Entity, _ int, mounted bool) {
	*e.changes = append(*e.changes, mounted)
}

func TestRidingOwnershipMovementInputAndRemoval(t *testing.T) {
	w := Config{Synchronous: true}.New()
	defer w.Close()
	other := Config{Synchronous: true}.New()
	defer other.Close()
	var inputs []RiderInput
	var changes []bool
	limit := 30.0
	typ := ridingTestType{seats: []Seat{{Position: mgl64.Vec3{1, 2, 0}, RotationLimit: &limit, Controlling: true}, {}}, inputs: &inputs, changes: &changes}
	vehicle := EntitySpawnOpts{Position: mgl64.Vec3{0, 4, 0}, Rotation: cube.Rotation{90, 0}}.New(typ, testEntityConfig{})
	rider := EntitySpawnOpts{Position: mgl64.Vec3{0, 4, 0}}.New(testEntityType{}, testEntityConfig{})
	passenger := EntitySpawnOpts{Position: mgl64.Vec3{0, 4, 0}}.New(testEntityType{}, testEntityConfig{})
	w.Do(func(tx *Tx) {
		v, r, p := tx.AddEntity(vehicle), tx.AddEntity(rider), tx.AddEntity(passenger)
		if !tx.Mount(r, v, 0) || tx.Mount(p, v, 0) || !tx.Mount(p, v, 1) {
			t.Fatal("seat occupancy was not enforced")
		}
		if r.Position().Sub(mgl64.Vec3{0, 6, 1}).Len() > 1e-9 {
			t.Fatalf("rotated rider position: %v", r.Position())
		}
		vehicle.data.Pos = mgl64.Vec3{5, 4, 5}
		tx.UpdateRiders(v)
		if r.Position().Sub(mgl64.Vec3{5, 6, 6}).Len() > 1e-9 {
			t.Fatalf("moving rider position: %v", r.Position())
		}
		if !tx.HandleRiderInput(r, RiderInput{Move: mgl64.Vec2{2, -3}, Yaw: 180}) || len(inputs) != 1 || inputs[0].Move != (mgl64.Vec2{1, -1}) {
			t.Fatal("controlling input not delivered or clamped")
		}
		if r.Rotation().Yaw() != 120 {
			t.Fatalf("rotation limit: %v", r.Rotation())
		}
		tx.HandleRiderInput(p, RiderInput{Move: mgl64.Vec2{1, 1}})
		tx.HandleRiderInput(r, RiderInput{Yaw: math.NaN()})
		if len(inputs) != 1 {
			t.Fatal("passenger or malformed input delivered")
		}
		if !tx.HandleRiderInput(r, RiderInput{Sneak: true}) {
			t.Fatal("dismount input not handled")
		}
		if _, _, ok := Riding(r); ok {
			t.Fatal("sneaking did not dismount")
		}
		if !tx.Mount(r, v, 0) {
			t.Fatal("seat not released")
		}
		tx.RemoveEntity(v)
		if _, _, ok := Riding(r); ok {
			t.Fatal("removed vehicle retained rider")
		}
		if _, _, ok := Riding(p); ok {
			t.Fatal("removed vehicle retained passenger")
		}
	})
	other.Do(func(tx *Tx) {
		v := tx.AddEntity(vehicle)
		if r, ok := rider.Entity(tx); ok || r != nil {
			t.Fatal("rider followed transfer")
		}
		if len(RiderLinks(v)) != 0 {
			t.Fatal("transferred vehicle retained old seats")
		}
	})
	if len(changes) != 6 {
		t.Fatalf("membership events: %v", changes)
	}
}

func TestMountRejectsCyclesAndOtherWorlds(t *testing.T) {
	w := Config{Synchronous: true}.New()
	defer w.Close()
	other := Config{Synchronous: true}.New()
	defer other.Close()
	var inputs []RiderInput
	var changes []bool
	typ := ridingTestType{seats: []Seat{{}}, inputs: &inputs, changes: &changes}
	a, b := NewEntity(typ, testEntityConfig{}), NewEntity(typ, testEntityConfig{})
	w.Do(func(tx *Tx) {
		x, y := tx.AddEntity(a), tx.AddEntity(b)
		if tx.Mount(x, x, 0) || tx.Mount(x, y, -1) || !tx.Mount(x, y, 0) || tx.Mount(y, x, 0) {
			t.Fatal("invalid mount accepted or valid mount rejected")
		}
		tx.RemoveEntity(x)
		if len(RiderLinks(y)) != 0 {
			t.Fatal("removed rider retained seat")
		}
	})
	other.Do(func(tx *Tx) {
		x := tx.AddEntity(a)
		if tx.Mount(x, &testEntity{handle: b}, 0) {
			t.Fatal("cross-world mount accepted")
		}
	})
}

// transientTestType opts one actor out of serialized terrain data.
type transientTestType struct{ testEntityType }

func (transientTestType) Transient() bool { return true }

func TestTransientActorsStayOutOfTerrainStorage(t *testing.T) {
	w := Config{Synchronous: true}.New()
	defer w.Close()
	w.Do(func(tx *Tx) {
		persistent := NewEntity(testEntityType{}, testEntityConfig{})
		transient := NewEntity(transientTestType{}, testEntityConfig{})
		column := w.columnTo(&Column{Entities: []*EntityHandle{persistent, transient}}, ChunkPos{})
		if len(column.Entities) != 1 || column.Entities[0].Data["identifier"] != persistent.Type().EncodeEntity() {
			t.Fatalf("saved transient actor: %v", column.Entities)
		}
	})
}

func TestSeatRotationLimitWrapsMultipleTurns(t *testing.T) {
	for _, test := range []struct{ vehicle, rider, want float64 }{
		{720, 10, 730}, {-720, -10, -730}, {1080, -1090, 1070}, {-1080, 1090, -1070},
	} {
		w := Config{Synchronous: true}.New()
		var inputs []RiderInput
		var changes []bool
		limit := 30.0
		typ := ridingTestType{seats: []Seat{{RotationLimit: &limit}}, inputs: &inputs, changes: &changes}
		vehicle := EntitySpawnOpts{Rotation: cube.Rotation{test.vehicle, 0}}.New(typ, testEntityConfig{})
		rider := EntitySpawnOpts{Rotation: cube.Rotation{test.rider, 0}}.New(testEntityType{}, testEntityConfig{})
		w.Do(func(tx *Tx) {
			v, r := tx.AddEntity(vehicle), tx.AddEntity(rider)
			if !tx.Mount(r, v, 0) || r.Rotation().Yaw() != test.want {
				t.Errorf("vehicle %v, rider %v: got %v, want %v", test.vehicle, test.rider, r.Rotation().Yaw(), test.want)
			}
		})
		w.Close()
	}
}
