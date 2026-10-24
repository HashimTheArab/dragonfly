package entity

import (
	"math"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

func TestProjectilePhysics_CapturedMovementPhases(t *testing.T) {
	// Independent live packet observations: official BDS 1.26.50.5 and a
	// genuine Dragonfly NewEnderPearl with only its launch speed set to 1.8.
	// Both throws hit the top of the same two-block pillar. These are airborne
	// positions at ticks 1, 2, 4, 6, 8, 10 and 12, excluding spawn/contact.
	for _, tc := range []struct {
		name            string
		start, velocity mgl64.Vec3
		positions       []mgl64.Vec3
		physics         ProjectilePhysics
		contact         mgl64.Vec3
	}{
		{
			name:  "bds",
			start: mgl64.Vec3{.5, 65.62001, .5}, velocity: mgl64.Vec3{1.301449e-7, .1839015, 1.4886842},
			positions: []mgl64.Vec3{
				{.5000001, 65.80391, 1.9886842},
				{.50000024, 65.962814, 3.4773684},
				{.5000005, 66.20562, 6.4547367},
				{.5000007, 66.34842, 9.432105},
				{.50000095, 66.39123, 12.409473},
				{.5000012, 66.33403, 15.386841},
				{.50000143, 66.176834, 18.364208},
			},
			physics: ProjectilePhysics{Speed: 1.5, Gravity: .025},
			contact: mgl64.Vec3{.5000016, 66, 20.493717},
		},
		{
			name:  "native_dragonfly",
			start: mgl64.Vec3{.5, 65.62, .5}, velocity: mgl64.Vec3{0, .23494714, 1.7846007},
			positions: []mgl64.Vec3{
				{.5, 65.8226, 2.2667546},
				{.5, 65.99317, 4.015842},
				{.5, 66.23951, 7.4617186},
				{.5, 66.36156, 10.839023},
				{.5, 66.36176, 14.149117},
				{.5, 66.24258, 17.393343},
				{.5, 66.006355, 20.573008},
			},
			physics: ProjectilePhysics{Speed: 1.8, Gravity: .03, Drag: .01, ForcesBeforeMove: true},
			contact: mgl64.Vec3{.5, 66, 20.634714},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			physics, ok := FitProjectilePhysics(tc.start, tc.velocity, tc.positions, 240)
			if !ok || physics.ForcesBeforeMove != tc.physics.ForcesBeforeMove ||
				math.Abs(physics.Speed-tc.physics.Speed) > 1e-4 ||
				math.Abs(physics.Gravity-tc.physics.Gravity) > 1e-4 ||
				math.Abs(physics.Drag-tc.physics.Drag) > 1e-4 {
				t.Fatalf("captured flight physics = %+v, accepted = %t; want %+v", physics, ok, tc.physics)
			}
			pillar := pearlPredictionBlocks{{0, 64, 20}: block.Stone{}, {0, 65, 20}: block.Stone{}}
			prediction, ok := PredictProjectileLanding(pillar, func(cube.Pos) bool { return true }, tc.start, tc.velocity, physics, 240)
			if !ok || !prediction.Hit || prediction.Block != (cube.Pos{0, 65, 20}) || prediction.Face != cube.FaceUp || prediction.Position.Sub(tc.contact).Len() > .001 {
				t.Fatalf("captured top contact prediction = %+v, accepted = %t; want %v", prediction, ok, tc.contact)
			}
		})
	}
}

func TestPredictProjectileLanding_ForcesBeforeDisplacement(t *testing.T) {
	// Drag halves the initial vertical velocity before gravity removes .25.
	// Applying gravity before drag, or moving before either force, differs.
	physics := ProjectilePhysics{Gravity: .25, Drag: .5, ForcesBeforeMove: true}
	got, ok := PredictProjectileLanding(pearlPredictionBlocks{}, func(cube.Pos) bool { return true }, mgl64.Vec3{.5, 10.5, .5}, mgl64.Vec3{2, 1, 3}, physics, 1)
	if !ok || got.Hit || got.Position != (mgl64.Vec3{1.5, 10.75, 2}) {
		t.Fatalf("first forced displacement = %+v, accepted = %t", got, ok)
	}
}

func TestProjectileCalibration_IndistinguishablePhaseKeepsBDSDefault(t *testing.T) {
	positions := []mgl64.Vec3{{1, 5, 1}, {2, 5, 2}, {3, 5, 3}, {5, 5, 5}, {8, 5, 8}, {12, 5, 12}}
	physics, ok := FitProjectilePhysics(mgl64.Vec3{0, 5, 0}, mgl64.Vec3{1, 0, 1}, positions, 240)
	if !ok || physics.ForcesBeforeMove || physics.Gravity != 0 || physics.Drag != 0 {
		t.Fatalf("force-free phase = %+v, accepted = %t; want BDS default", physics, ok)
	}
}

func TestProjectileCalibration_RejectsAmbiguousForcePhase(t *testing.T) {
	// A short late-flight window can be explained by either phase with a
	// slightly different gravity. These are independent closed-form samples
	// at ticks 100, 101, 102, 104 and 106 with forces before displacement.
	// Their launch velocity is known, but the observation precision cannot
	// establish the force phase. Do not publish either incompatible model.
	positions := []mgl64.Vec3{
		{100, 64.61, 0},
		{101, 64.5898, 0},
		{102, 64.5694, 0},
		{104, 64.528, 0},
		{106, 64.4858, 0},
	}
	if physics, ok := FitProjectilePhysics(mgl64.Vec3{0, 65.62, 0}, mgl64.Vec3{1, 0, 0}, positions, 240); ok {
		t.Fatalf("ambiguous force phase accepted: %+v", physics)
	}
}
