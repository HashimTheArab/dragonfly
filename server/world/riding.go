package world

import (
	"math"
	"slices"

	"github.com/go-gl/mathgl/mgl64"
)

// Seat describes a rider's position relative to its vehicle, in blocks.
type Seat struct {
	Position mgl64.Vec3
	// RotationLimit is the permitted yaw either side of the vehicle's yaw.
	// A nil limit allows the rider to look around freely.
	RotationLimit *float64
	// Controlling delivers rider input to the vehicle.
	Controlling bool
}

// RideableEntityType supplies the seats available on an entity type.
type RideableEntityType interface {
	EntityType
	Seats() []Seat
}

// RiderInput is an input observation, not permission to move the vehicle.
type RiderInput struct {
	Move        mgl64.Vec2
	Yaw, Pitch  float64
	Jump, Sneak bool
}

// RiderInputHandler receives validated input in the vehicle's transaction.
type RiderInputHandler interface {
	HandleRiderInput(rider Entity, seat int, input RiderInput)
}

// RiderChangeHandler observes committed mount and dismount changes.
type RiderChangeHandler interface {
	HandleRiderChange(rider Entity, seat int, mounted bool)
}

// EntityLinkAction publishes a changed rider link through the normal viewer path.
type EntityLinkAction struct {
	Rider  *EntityHandle
	Seat   Seat
	Remove bool
}

// EntityAction marks EntityLinkAction as an entity action.
func (EntityLinkAction) EntityAction() {}

// Riding returns the vehicle and seat of e. Call it only within e's transaction.
func Riding(e Entity) (*EntityHandle, Seat, bool) {
	d := &e.H().data
	if d.vehicle == nil {
		return nil, Seat{}, false
	}
	return d.vehicle, d.seat, true
}

// RiderLink is one occupied seat, ordered by its seat index by RiderLinks.
type RiderLink struct {
	Rider *EntityHandle
	Seat  Seat
}

// RiderLinks returns occupied seats for late viewers in e's transaction.
func RiderLinks(e Entity) []RiderLink {
	indices := make([]int, 0, len(e.H().data.riders))
	for index := range e.H().data.riders {
		indices = append(indices, index)
	}
	slices.Sort(indices)
	out := make([]RiderLink, 0, len(indices))
	for _, index := range indices {
		h := e.H().data.riders[index]
		out = append(out, RiderLink{Rider: h, Seat: h.data.seat})
	}
	return out
}

// Mount assigns rider to an unoccupied seat in the same world. It rejects cycles.
func (tx *Tx) Mount(rider, vehicle Entity, index int) bool {
	tx.rejectDetached()
	if rider == nil || vehicle == nil || rider.H() == vehicle.H() ||
		rider.H().w != tx.World() || vehicle.H().w != tx.World() {
		return false
	}
	t, ok := vehicle.H().Type().(RideableEntityType)
	if !ok || index < 0 || index >= len(t.Seats()) {
		return false
	}
	for h := vehicle.H(); h != nil; h = h.data.vehicle {
		if h == rider.H() {
			return false
		}
	}
	vd, rd := &vehicle.H().data, &rider.H().data
	if rd.vehicle != nil || (vd.riders != nil && vd.riders[index] != nil) {
		return false
	}
	if vd.riders == nil {
		vd.riders = make(map[int]*EntityHandle)
	}
	seat := t.Seats()[index]
	for _, n := range seat.Position {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return false
		}
	}
	if limit := seat.RotationLimit; limit != nil {
		if math.IsNaN(*limit) || math.IsInf(*limit, 0) || *limit < 0 {
			return false
		}
		value := *limit
		seat.RotationLimit = &value
	}
	vd.riders[index] = rider.H()
	rd.vehicle, rd.seat, rd.seatIndex = vehicle.H(), seat, index
	tx.UpdateRiders(vehicle)
	for _, v := range tx.ridingViewers(rider, vehicle) {
		v.ViewEntityState(rider)
		v.ViewEntityAction(vehicle, EntityLinkAction{Rider: rider.H(), Seat: seat})
	}
	if handler, ok := vehicle.(RiderChangeHandler); ok {
		handler.HandleRiderChange(rider, index, true)
	}
	return true
}

// Dismount detaches rider before either entity leaves the world.
func (tx *Tx) Dismount(rider Entity) bool {
	tx.rejectDetached()
	if rider == nil {
		return false
	}
	rd := &rider.H().data
	if rd.vehicle == nil || rider.H().w != tx.World() {
		return false
	}
	vehicle, ok := rd.vehicle.Entity(tx)
	if !ok {
		return false
	}
	index, seat := rd.seatIndex, rd.seat
	delete(vehicle.H().data.riders, index)
	rd.vehicle, rd.seat, rd.seatIndex = nil, Seat{}, 0
	for _, v := range tx.ridingViewers(rider, vehicle) {
		v.ViewEntityAction(vehicle, EntityLinkAction{Rider: rider.H(), Seat: seat, Remove: true})
		v.ViewEntityState(rider)
	}
	if handler, ok := vehicle.(RiderChangeHandler); ok {
		handler.HandleRiderChange(rider, index, false)
	}
	return true
}

// UpdateRiders moves passengers after the vehicle's authoritative movement.
func (tx *Tx) UpdateRiders(vehicle Entity) {
	tx.rejectDetached()
	for _, h := range vehicle.H().data.riders {
		rider, ok := h.Entity(tx)
		if !ok {
			continue
		}
		d := &h.data
		yaw := vehicle.Rotation().Yaw() * math.Pi / 180
		offset := d.seat.Position
		offset = mgl64.Vec3{offset.X()*math.Cos(yaw) - offset.Z()*math.Sin(yaw), offset.Y(), offset.X()*math.Sin(yaw) + offset.Z()*math.Cos(yaw)}
		d.Pos, d.Vel = vehicle.Position().Add(offset), vehicle.H().data.Vel
		if limit := d.seat.RotationLimit; limit != nil {
			delta := math.Mod(d.Rot.Yaw()-vehicle.Rotation().Yaw()+540, 360) - 180
			d.Rot[0] = vehicle.Rotation().Yaw() + max(-*limit, min(*limit, delta))
		}
		for _, v := range tx.ridingViewers(rider, vehicle) {
			v.ViewEntityMovement(rider, d.Pos, d.Rot, false)
		}
		tx.UpdateRiders(rider)
	}
}

// HandleRiderInput delivers finite, clamped input only to the rider's current vehicle.
func (tx *Tx) HandleRiderInput(rider Entity, input RiderInput) bool {
	tx.rejectDetached()
	d := &rider.H().data
	if d.vehicle == nil || rider.H().w != tx.World() {
		return false
	}
	if input.Sneak {
		tx.Dismount(rider)
		return true
	}
	for _, n := range []float64{input.Move.X(), input.Move.Y(), input.Yaw, input.Pitch} {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return true
		}
	}
	d.Rot[0], d.Rot[1] = input.Yaw, max(-90, min(90, input.Pitch))
	vehicle, ok := d.vehicle.Entity(tx)
	if !ok {
		return true
	}
	tx.UpdateRiders(vehicle)
	input.Move[0], input.Move[1] = max(-1, min(1, input.Move.X())), max(-1, min(1, input.Move.Y()))
	if handler, ok := vehicle.(RiderInputHandler); ok && d.seat.Controlling {
		handler.HandleRiderInput(rider, d.seatIndex, input)
	}
	return true
}

// ridingViewers includes the controlling viewer and deduplicates chunk viewers.
func (tx *Tx) ridingViewers(rider, vehicle Entity) []Viewer {
	out := slices.Clone(tx.Viewers(vehicle.Position()))
	for _, v := range tx.Viewers(rider.Position()) {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// detachRiders removes both incoming and outgoing links before despawning an entity.
func (tx *Tx) detachRiders(e Entity) {
	tx.Dismount(e)
	for _, link := range RiderLinks(e) {
		if rider, ok := link.Rider.Entity(tx); ok {
			tx.Dismount(rider)
		}
	}
}
