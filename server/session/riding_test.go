package session

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// ridingEntityType opens a small entity for replication tests.
type ridingEntityType struct{ networkEncodedOffsetType }

func (ridingEntityType) Seats() []world.Seat {
	return []world.Seat{{Position: mgl64.Vec3{0, 1, 0}, Controlling: true}}
}
func (ridingEntityType) Open(_ *world.Tx, h *world.EntityHandle, d *world.EntityData) world.Entity {
	return ridingEntity{h: h, d: d}
}

// ridingEntity exposes transaction-owned state without a real connection.
type ridingEntity struct {
	h *world.EntityHandle
	d *world.EntityData
}

func (e ridingEntity) H() *world.EntityHandle  { return e.h }
func (e ridingEntity) Position() mgl64.Vec3    { return e.d.Pos }
func (e ridingEntity) Rotation() cube.Rotation { return e.d.Rot }
func (ridingEntity) Close() error              { return nil }
func (ridingEntity) EntityProperties() protocol.EntityProperties {
	return protocol.EntityProperties{IntegerProperties: []protocol.IntegerEntityProperty{{Index: 0, Value: 7}}}
}

// ridingConfig leaves the spawn options untouched.
type ridingConfig struct{}

func (ridingConfig) Apply(*world.EntityData) {}

// ridingSession captures packets from the normal session viewer.
func ridingSession() *Session {
	return &Session{packets: make(chan packet.Packet, 64), closeBackground: make(chan struct{}), entityRuntimeIDs: make(map[*world.EntityHandle]uint64), entities: make(map[uint64]*world.EntityHandle)}
}

// drainRidingPackets returns all immediately available viewer output.
func drainRidingPackets(s *Session) []packet.Packet {
	var out []packet.Packet
	for {
		select {
		case p := <-s.packets:
			out = append(out, p)
		default:
			return out
		}
	}
}

func TestRidingLinksWaitForBothActorsAndRestoreForLateViewer(t *testing.T) {
	w := world.Config{Synchronous: true}.New()
	defer w.Close()
	vh, rh := world.NewEntity(ridingEntityType{}, ridingConfig{}), world.NewEntity(ridingEntityType{}, ridingConfig{})
	w.Do(func(tx *world.Tx) {
		vehicle, rider := tx.AddEntity(vh), tx.AddEntity(rh)
		if !tx.Mount(rider, vehicle, 0) {
			t.Fatal("mount failed")
		}
		for _, order := range [][]world.Entity{{vehicle, rider}, {rider, vehicle}} {
			s := ridingSession()
			s.ViewEntity(order[0])
			first := drainRidingPackets(s)
			if len(first) != 1 {
				t.Fatalf("link emitted before both actors: %v", first)
			}
			s.ViewEntity(order[1])
			packets := drainRidingPackets(s)
			if len(packets) != 2 {
				t.Fatalf("late spawn output: %v", packets)
			}
			if _, ok := packets[0].(*packet.AddActor); !ok {
				t.Fatalf("link precedes second actor: %T", packets[0])
			}
			link, ok := packets[1].(*packet.SetActorLink)
			if !ok || link.EntityLink.RiddenEntityUniqueID != int64(s.handleRuntimeID(vh)) || link.EntityLink.RiderEntityUniqueID != int64(s.handleRuntimeID(rh)) || link.EntityLink.Type != protocol.EntityLinkRider {
				t.Fatalf("wrong link: %+v", packets[1])
			}
			metadata := s.entityMetadata(rider)
			if metadata[protocol.EntityDataKeySeatOffset] != vec64To32(mgl64.Vec3{0, 1, 0}) {
				t.Fatalf("seat offset: %v", metadata)
			}
			s.ViewEntityState(vehicle)
			data := drainRidingPackets(s)[0].(*packet.SetActorData)
			if len(data.EntityProperties.IntegerProperties) != 1 || data.EntityProperties.IntegerProperties[0].Value != 7 {
				t.Fatal("properties not replicated")
			}
			s.ViewEntityAction(vehicle, world.EntityLinkAction{Rider: rh, Remove: true})
			removed := drainRidingPackets(s)[0].(*packet.SetActorLink).EntityLink
			if removed.Type != protocol.EntityLinkRemove || !removed.Immediate {
				t.Fatalf("dismount: %+v", removed)
			}
		}
	})
}

func TestActorIdentifiersUseNetworkIdentity(t *testing.T) {
	reg := world.EntityRegistryConfig{}.New([]world.EntityType{networkEncodedOffsetType{}})
	ids := availableEntityIdentifiers(reg)
	if len(ids) != 1 || ids[0].ID != "minecraft:minecart" {
		t.Fatalf("identifiers: %v", ids)
	}
}

// baseActorType keeps its custom identity while selecting an existing client actor class.
type baseActorType struct{ networkEncodedOffsetType }

// NetworkEncodeEntity advertises a distinct identity for the custom actor.
func (baseActorType) NetworkEncodeEntity() string { return "test:vehicle" }

// BaseEntityIdentifier selects the existing actor class for client construction.
func (baseActorType) BaseEntityIdentifier() string { return "minecraft:boat" }

func TestActorIdentifiersRetainBaseIdentity(t *testing.T) {
	reg := world.EntityRegistryConfig{}.New([]world.EntityType{baseActorType{}})
	data, err := nbt.Marshal(map[string]any{"idlist": availableEntityIdentifiers(reg)})
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		IDs []map[string]any `nbt:"idlist"`
	}
	if err := nbt.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.IDs) != 1 || decoded.IDs[0]["id"] != "test:vehicle" || decoded.IDs[0]["bid"] != "minecraft:boat" {
		t.Fatalf("actor base identity lost: %v", decoded.IDs)
	}
}

// inputVehicleType opens a vehicle whose input handler records packet observations.
type inputVehicleType struct {
	ridingEntityType
	input *[]world.RiderInput
}

func (t inputVehicleType) Open(_ *world.Tx, h *world.EntityHandle, d *world.EntityData) world.Entity {
	return inputVehicle{ridingEntity{h, d}, t.input}
}

// inputVehicle records input on the world transaction.
type inputVehicle struct {
	ridingEntity
	input *[]world.RiderInput
}

func (v inputVehicle) HandleRiderInput(_ world.Entity, _ int, input world.RiderInput) {
	*v.input = append(*v.input, input)
}

// inputRider implements only the controllable operations used by an idle riding packet.
type inputRider struct {
	Controllable
	entity ridingEntity
	dead   bool
}

func (r inputRider) H() *world.EntityHandle  { return r.entity.H() }
func (r inputRider) Position() mgl64.Vec3    { return r.entity.Position() }
func (r inputRider) Rotation() cube.Rotation { return r.entity.Rotation() }
func (r inputRider) Close() error            { return nil }

// Dead supplies the controller state independently of the vehicle.
func (r inputRider) Dead() bool { return r.dead }

func (inputRider) Sneaking() bool { return false }

func TestAuthInputDeliversSteeringWithoutTrustingVehiclePosition(t *testing.T) {
	w := world.Config{Synchronous: true}.New()
	defer w.Close()
	var input []world.RiderInput
	vh := world.NewEntity(inputVehicleType{input: &input}, ridingConfig{})
	rh := world.NewEntity(ridingEntityType{}, ridingConfig{})
	w.Do(func(tx *world.Tx) {
		vehicle, rider := tx.AddEntity(vh), tx.AddEntity(rh)
		if !tx.Mount(rider, vehicle, 0) {
			t.Fatal("mount failed")
		}
		packet := &packet.PlayerAuthInput{MoveVector: mgl32.Vec2{0.5, 1}, Yaw: 23, Pitch: 4, Position: mgl32.Vec3{999, 999, 999}}
		if err := (PlayerAuthInputHandler{}).Handle(packet, ridingSession(), tx, inputRider{entity: rider.(ridingEntity)}); err != nil {
			t.Fatal(err)
		}
		if len(input) != 1 || input[0].Move != (mgl64.Vec2{0.5, 1}) || input[0].Yaw != 23 {
			t.Fatalf("steering: %v", input)
		}
		if vehicle.Position() != (mgl64.Vec3{}) || rider.Position() != (mgl64.Vec3{0, 1, 0}) {
			t.Fatal("client vehicle position became authoritative")
		}
	})
}

func TestDeadControllerInputDoesNotSteer(t *testing.T) {
	w := world.Config{Synchronous: true}.New()
	defer w.Close()
	var input []world.RiderInput
	vh := world.NewEntity(inputVehicleType{input: &input}, ridingConfig{})
	rh := world.NewEntity(ridingEntityType{}, ridingConfig{})
	w.Do(func(tx *world.Tx) {
		vehicle, rider := tx.AddEntity(vh), tx.AddEntity(rh)
		if !tx.Mount(rider, vehicle, 0) {
			t.Fatal("mount failed")
		}
		before := rider.Rotation()
		pk := &packet.PlayerAuthInput{MoveVector: mgl32.Vec2{1, 1}, Yaw: 90, Pitch: 20, Tick: 42}
		s := ridingSession()
		if err := (PlayerAuthInputHandler{}).Handle(pk, s, tx, inputRider{entity: rider.(ridingEntity), dead: true}); err != nil {
			t.Fatal(err)
		}
		if len(input) != 0 || rider.Rotation() != before {
			t.Fatal("dead controller input reached vehicle or changed rider rotation")
		}
		if s.inputTick.Load() != 42 {
			t.Fatal("dead input lost its packet tick")
		}
	})
}

// teleportRiderType supplies the rider's session to the world's link notifications.
type teleportRiderType struct {
	ridingEntityType
	session *Session
}

// Open opens a rider that forwards link changes to its controlling session.
func (t teleportRiderType) Open(_ *world.Tx, h *world.EntityHandle, d *world.EntityData) world.Entity {
	return teleportRider{ridingEntity{h, d}, t.session}
}

// teleportRider exposes a controlling viewer without a live connection.
type teleportRider struct {
	ridingEntity
	session *Session
}

// ControllingViewer includes the session in authoritative riding updates.
func (r teleportRider) ControllingViewer() world.Viewer { return r.session }

// Move applies accepted on-foot input to the test rider's transaction-owned data.
func (r inputRider) Move(delta mgl64.Vec3, yaw, pitch float64) {
	r.entity.d.Pos = r.entity.d.Pos.Add(delta)
	r.entity.d.Rot = r.entity.d.Rot.Add(cube.Rotation{yaw, pitch})
}

func TestTeleportMountRideDismountAllowsMovement(t *testing.T) {
	for _, knownVehicle := range []bool{true, false} {
		t.Run(map[bool]string{true: "visible vehicle", false: "unseen vehicle"}[knownVehicle], func(t *testing.T) {
			w := world.Config{Synchronous: true}.New()
			defer w.Close()
			s := ridingSession()
			var input []world.RiderInput
			vh := world.NewEntity(inputVehicleType{input: &input}, ridingConfig{})
			rh := world.NewEntity(teleportRiderType{session: s}, ridingConfig{})
			s.ent = rh
			s.entityRuntimeIDs[rh] = selfEntityRuntimeID
			if knownVehicle {
				s.entityRuntimeIDs[vh] = 2
			}
			w.Do(func(tx *world.Tx) {
				vehicle := tx.AddEntity(vh).(inputVehicle)
				rider := tx.AddEntity(rh).(teleportRider)
				c := inputRider{entity: rider.ridingEntity}
				destination := mgl64.Vec3{100, 64, 100}
				rider.d.Pos = destination
				s.ViewEntityTeleport(c, destination)
				vehicle.d.Pos = destination
				if !tx.Mount(rider, vehicle, 0) {
					t.Fatal("mount failed")
				}
				vehicle.d.Pos = destination.Add(mgl64.Vec3{10, 0, 0})
				if err := (PlayerAuthInputHandler{}).Handle(&packet.PlayerAuthInput{Position: mgl32.Vec3{999, 999, 999}}, s, tx, c); err != nil {
					t.Fatal(err)
				}
				if len(input) != 1 {
					t.Fatal("riding input not delivered")
				}
				if rider.Position() != vehicle.Position().Add(mgl64.Vec3{0, 1, 0}) {
					t.Fatal("rider left authoritative seat position")
				}
				if !tx.Dismount(rider) {
					t.Fatal("dismount failed")
				}
				target := rider.Position().Add(mgl64.Vec3{0.5, 0, 0})
				pk := &packet.PlayerAuthInput{Position: vec64To32(entityNetworkPosition(c, target))}
				if err := (PlayerAuthInputHandler{}).Handle(pk, s, tx, c); err != nil {
					t.Fatal(err)
				}
				if rider.Position().Sub(target).Len() > 0.0001 {
					t.Fatalf("movement after dismount rejected: got %v, want %v", rider.Position(), target)
				}
			})
		})
	}
}
