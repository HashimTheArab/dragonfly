package entity

import (
	"math"
	"testing"

	"github.com/go-gl/mathgl/mgl64"
)

func TestProjectileCalibration_IrregularObservations(t *testing.T) {
	start := mgl64.Vec3{17.5, 65.62001, -9.5}
	for _, test := range []struct {
		name          string
		velocity      mgl64.Vec3
		gravity, drag float64
		ticks         []int
	}{
		{"bds", mgl64.Vec3{0, 0, 1.5}, .025, 0, []int{1, 2, 4, 7, 11, 18, 27}},
		{"custom", mgl64.Vec3{.733, .619, 1.101}, .0367, .0273, []int{1, 2, 4, 7, 11, 18, 27}},
		{"strong_drag", mgl64.Vec3{.7, .6, 1.2}, .084, .12, []int{1, 2, 3, 5, 8, 12, 18}},
		{"no_forces", mgl64.Vec3{1.2, .4, .9}, 0, 0, []int{1, 3, 6, 10, 18, 25}},
	} {
		t.Run(test.name, func(t *testing.T) {
			positions := calibrationObservations(start, test.velocity, test.gravity, test.drag, test.ticks)
			physics, ok := FitProjectilePhysics(start, test.velocity, positions, 240)
			if !ok {
				t.Fatal("independent irregular flight was rejected")
			}
			if math.Abs(physics.Speed-test.velocity.Len()) > 1e-5 || math.Abs(physics.Gravity-test.gravity) > 1e-4 || math.Abs(physics.Drag-test.drag) > 1e-4 {
				t.Fatalf("physics = %+v, want speed %g, gravity %g, drag %g", physics, test.velocity.Len(), test.gravity, test.drag)
			}
		})
	}
}

func TestProjectileCalibration_Float32CoordinateDrift(t *testing.T) {
	// Golden float32 observations at ticks 1, 10, 20, 30, 50, 70 and 90.
	// Coordinate rounding changes the apparent horizontal direction even
	// though the projectile has constant horizontal velocity and zero drag.
	start := mgl64.Vec3{4000.5, 65.62000274658203, 4000.5}
	velocity := mgl64.Vec3{1.2000000476837158, .4000000059604645, .8999999761581421}
	positions := []mgl64.Vec3{
		{4001.699951171875, 66.02000427246094, 4001.39990234375},
		{4012.49951171875, 68.49500274658203, 4009.4990234375},
		{4024.4990234375, 68.87000274658203, 4018.498046875},
		{4036.49853515625, 66.74500274658203, 4027.4970703125},
		{4060.49755859375, 54.99500274658203, 4045.4951171875},
		{4084.49658203125, 33.24501037597656, 4063.4931640625},
		{4108.498046875, 1.4950299263000488, 4081.4912109375},
	}
	physics, ok := FitProjectilePhysics(start, velocity, positions, 240)
	if !ok || math.Abs(physics.Gravity-.025) > 1e-4 || math.Abs(physics.Drag) > 1e-4 {
		t.Fatalf("rounded flight physics = %+v, accepted = %t; want gravity .025, drag 0", physics, ok)
	}
}

func TestProjectileCalibration_RejectsUnreliableObservations(t *testing.T) {
	start, velocity := mgl64.Vec3{.5, 65.62, .5}, mgl64.Vec3{.7, .6, 1.2}
	valid := calibrationObservations(start, velocity, .04, .03, []int{1, 2, 3, 5, 8, 13, 20})
	teleported := append([]mgl64.Vec3(nil), valid...)
	teleported[len(teleported)-1][0] += 200
	deflected := append([]mgl64.Vec3(nil), valid...)
	for i := 4; i < len(deflected); i++ {
		deflected[i][0] += float64(i-3) * .7
	}
	reordered := append([]mgl64.Vec3(nil), valid...)
	reordered[4], reordered[5] = reordered[5], reordered[4]
	nonFiniteSample := append([]mgl64.Vec3(nil), valid...)
	nonFiniteSample[3][1] = math.NaN()
	for _, test := range []struct {
		name            string
		start, velocity mgl64.Vec3
		positions       []mgl64.Vec3
		maxTicks        int
	}{
		{"no_observations", start, velocity, nil, 240},
		{"endpoint_only", start, velocity, valid[len(valid)-1:], 240},
		{"short_flight", start, velocity, calibrationObservations(start, velocity, .04, .03, []int{1, 2, 3, 4, 5}), 240},
		{"duplicate_positions", start, velocity, []mgl64.Vec3{valid[0], valid[0], valid[0], valid[0], valid[0], valid[0]}, 240},
		{"vertical_flight", start, mgl64.Vec3{0, 1.5, 0}, calibrationObservations(start, mgl64.Vec3{0, 1.5, 0}, .025, 0, []int{1, 2, 4, 7, 11, 18}), 240},
		{"non_finite_start", mgl64.Vec3{math.NaN(), 65.62, .5}, velocity, valid, 240},
		{"non_finite_velocity", start, mgl64.Vec3{.7, math.Inf(1), 1.2}, valid, 240},
		{"non_finite_observation", start, velocity, nonFiniteSample, 240},
		{"no_tick_budget", start, velocity, valid, 0},
		{"teleport_discontinuity", start, velocity, teleported, 240},
		{"sideways_deflection", start, velocity, deflected, 240},
		{"out_of_order", start, velocity, reordered, 240},
	} {
		t.Run(test.name, func(t *testing.T) {
			if physics, ok := FitProjectilePhysics(test.start, test.velocity, test.positions, test.maxTicks); ok {
				t.Fatalf("unreliable flight accepted with physics %+v", physics)
			}
		})
	}
}

// calibrationObservations evaluates the discrete motion's closed form in
// double precision, then quantizes only the transmitted positions. It does not
// call the predictor or fitter, and deliberately omits intermediate ticks.
func calibrationObservations(start, velocity mgl64.Vec3, gravity, drag float64, ticks []int) []mgl64.Vec3 {
	positions := make([]mgl64.Vec3, 0, len(ticks))
	for _, tick := range ticks {
		travel, gravityTravel := float64(tick), float64(tick*(tick-1))/2
		if drag != 0 {
			travel = (1 - math.Pow(1-drag, float64(tick))) / drag
			gravityTravel = (float64(tick) - travel) / drag
		}
		position := start.Add(velocity.Mul(travel))
		position[1] -= gravity * gravityTravel
		for axis := range position {
			position[axis] = float64(float32(position[axis]))
		}
		positions = append(positions, position)
	}
	return positions
}
