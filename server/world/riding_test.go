package world

import (
	"math"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/go-gl/mathgl/mgl64"
)

// controlledRiderType opens a rider with a viewer independent of chunk loading.
type controlledRiderType struct {
	testEntityType
	viewer Viewer
}

// Open binds the controlling viewer to the transaction-scoped rider.
func (t controlledRiderType) Open(_ *Tx, h *EntityHandle, d *EntityData) Entity {
	return &controlledRider{testEntity: testEntity{handle: h, data: d}, viewer: t.viewer}
}

// controlledRider exposes its controlling viewer without a chunk loader.
type controlledRider struct {
	testEntity
	viewer Viewer
}

// ControllingViewer returns the rider's own viewer.
func (e *controlledRider) ControllingViewer() Viewer { return e.viewer }

// ridingViewer records rider updates that must reach a controlling client.
type ridingViewer struct {
	NopViewer
	moves, states int
	links         []EntityLinkAction
}

// ViewEntityMovement records seat movement.
func (v *ridingViewer) ViewEntityMovement(Entity, mgl64.Vec3, cube.Rotation, bool) { v.moves++ }

// ViewEntityState records rider metadata updates.
func (v *ridingViewer) ViewEntityState(Entity) { v.states++ }

// ViewEntityAction records mount and dismount links.
func (v *ridingViewer) ViewEntityAction(_ Entity, action EntityAction) {
	if link, ok := action.(EntityLinkAction); ok {
		v.links = append(v.links, link)
	}
}

func TestRidingNotifiesControllingViewerOutsideLoadedChunks(t *testing.T) {
	for _, loaded := range []bool{false, true} {
		w := Config{Synchronous: true}.New()
		viewer := &ridingViewer{}
		var inputs []RiderInput
		var changes []bool
		vehicle := EntitySpawnOpts{Position: mgl64.Vec3{8, 70, 8}}.New(ridingTestType{seats: []Seat{{}}, inputs: &inputs, changes: &changes}, testEntityConfig{})
		rider := NewEntity(controlledRiderType{viewer: viewer}, testEntityConfig{})
		w.Do(func(tx *Tx) {
			v, r := tx.AddEntity(vehicle), tx.AddEntity(rider)
			if loaded {
				loader := NewLoader(0, w, viewer)
				loader.Move(tx, v.Position())
				loader.Load(tx, 1)
				viewer.moves, viewer.states, viewer.links = 0, 0, nil
			}
			if !tx.Mount(r, v, 0) {
				t.Fatal("mount failed")
			}
			vehicle.data.Pos = mgl64.Vec3{1024, 80, 1024}
			tx.UpdateRiders(v)
			if !tx.Dismount(r) {
				t.Fatal("dismount failed")
			}
			if viewer.moves != 2 || viewer.states != 2 || len(viewer.links) != 2 || viewer.links[0].Remove || !viewer.links[1].Remove {
				t.Fatalf("controlling viewer missed or duplicated riding updates: %+v", viewer)
			}
		})
		w.Close()
	}
}

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

func TestSeatPositionUsesReducedFiniteYaw(t *testing.T) {
	w := Config{Synchronous: true}.New()
	defer w.Close()
	var inputs []RiderInput
	var changes []bool
	for _, offset := range []mgl64.Vec3{{}, {1, 2, 3}} {
		typ := ridingTestType{seats: []Seat{{Position: offset}}, inputs: &inputs, changes: &changes}
		vehicle := EntitySpawnOpts{Position: mgl64.Vec3{8, 70, 8}, Rotation: cube.Rotation{1e308, 0}}.New(typ, testEntityConfig{})
		rider := NewEntity(testEntityType{}, testEntityConfig{})
		w.Do(func(tx *Tx) {
			v, r := tx.AddEntity(vehicle), tx.AddEntity(rider)
			if !tx.Mount(r, v, 0) {
				t.Fatal("finite seat rejected")
			}
			yaw := math.Mod(1e308, 360) * math.Pi / 180
			want := v.Position().Add(mgl64.Vec3{offset.X()*math.Cos(yaw) - offset.Z()*math.Sin(yaw), offset.Y(), offset.X()*math.Sin(yaw) + offset.Z()*math.Cos(yaw)})
			if difference := r.Position().Sub(want).Len(); math.IsNaN(difference) || difference > 1e-9 {
				t.Fatalf("seat position %v differs from reduced heading %v", r.Position(), want)
			}
		})
	}
}

func TestSeatPositionRejectsUnrepresentableCoordinates(t *testing.T) {
	vehicle := &testEntity{data: &EntityData{Pos: mgl64.Vec3{math.MaxFloat64, 0, 0}}}
	if _, ok := SeatPosition(vehicle.Position(), 0, Seat{Position: mgl64.Vec3{math.MaxFloat64, 0, 0}}); ok {
		t.Fatal("overflowed seat coordinates accepted")
	}
}

func TestUpdatedRiderPositionAndRotationAreSaved(t *testing.T) {
	provider := &columnSaveObserver{stored: make(map[ChunkPos]*chunk.Column)}
	w := Config{Synchronous: true, Provider: provider}.New()
	defer w.Close()
	var inputs []RiderInput
	var changes []bool
	limit := 0.0
	typ := ridingTestType{seats: []Seat{{Position: mgl64.Vec3{0, 1, 0}, RotationLimit: &limit}}, inputs: &inputs, changes: &changes}
	vehicle := EntitySpawnOpts{Position: mgl64.Vec3{8, 70, 8}}.New(typ, testEntityConfig{})
	rider := EntitySpawnOpts{}.New(testEntityType{}, testEntityConfig{})
	w.Do(func(tx *Tx) {
		v, r := tx.AddEntity(vehicle), tx.AddEntity(rider)
		if !tx.Mount(r, v, 0) {
			t.Fatal("mount failed")
		}
		// Model a resident column whose saved entities have not changed since load.
		w.chunks[ChunkPos{}].modified = false
		vehicle.data.Pos = mgl64.Vec3{10, 72, 10}
		vehicle.data.Rot = cube.Rotation{90, 0}
		tx.UpdateRiders(v)
	})
	w.Save()
	saved := provider.stored[ChunkPos{}]
	if saved == nil {
		t.Fatal("updated rider column was not saved")
	}
	for _, e := range saved.Entities {
		if readVec3(e.Data, "Pos") == (mgl64.Vec3{10, 73, 10}) && readRotation(e.Data) == (cube.Rotation{90, 0}) {
			return
		}
	}
	t.Fatalf("saved entities lost rider position or rotation: %v", saved.Entities)
}
