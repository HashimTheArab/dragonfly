package entity

import (
	"math"
	"testing"

	"github.com/go-gl/mathgl/mgl64"
)

// projectileClosestApproach returns how near a flight from the origin with
// velocity passes target across its first ticks, following each tick's
// displacement in a straight line.
func projectileClosestApproach(t *testing.T, velocity, target mgl64.Vec3, physics ProjectilePhysics, ticks int) (float64, int) {
	t.Helper()
	best, bestTick := math.Inf(1), 0
	previous := mgl64.Vec3{}
	for tick, position := range projectileFlightPositions(t, mgl64.Vec3{}, velocity, physics, ticks) {
		segment := position.Sub(previous)
		share := 0.0
		if length := segment.LenSqr(); length > 0 {
			share = math.Max(0, math.Min(1, target.Sub(previous).Dot(segment)/length))
		}
		if distance := previous.Add(segment.Mul(share)).Sub(target).Len(); distance < best {
			best, bestTick = distance, tick+1
		}
		previous = position
	}
	return best, bestTick
}

func TestProjectileLaunchToward_ReachesTargetAtSpeed(t *testing.T) {
	pearl := ProjectilePhysics{Speed: 1.5, Gravity: 0.025}
	snowball := ProjectilePhysics{Speed: 1.5, Gravity: 0.03, Drag: projectileAirDrag}
	native := ProjectilePhysics{Speed: 1.5, Gravity: 0.03, Drag: projectileAirDrag, ForcesBeforeMove: true}
	for _, tc := range []struct {
		name    string
		target  mgl64.Vec3
		physics ProjectilePhysics
	}{
		{"pearl level", mgl64.Vec3{0, 0, 20}, pearl},
		{"pearl below", mgl64.Vec3{12, -6, -9}, pearl},
		{"pearl above", mgl64.Vec3{-5, 4, 3}, pearl},
		{"pearl straight down", mgl64.Vec3{0, -10, 0}, pearl},
		{"snowball drag", mgl64.Vec3{18, -2, 7}, snowball},
		{"forces before move", mgl64.Vec3{-14, 1, 6}, native},
	} {
		t.Run(tc.name, func(t *testing.T) {
			velocity, ok := ProjectileLaunchToward(tc.target, tc.physics, 200)
			if !ok {
				t.Fatal("no launch found")
			}
			if got := velocity.Len(); math.Abs(got-tc.physics.Speed) > 1e-9 {
				t.Fatalf("launch speed = %v, want %v", got, tc.physics.Speed)
			}
			if miss, _ := projectileClosestApproach(t, velocity, tc.target, tc.physics, 200); miss > 0.05 {
				t.Fatalf("flight misses the target by %v", miss)
			}
		})
	}
}

func TestProjectileLaunchToward_ChoosesTheFlattestArc(t *testing.T) {
	physics := ProjectilePhysics{Speed: 1.5, Gravity: 0.025}
	target := mgl64.Vec3{0, 0, 30}
	velocity, ok := ProjectileLaunchToward(target, physics, 200)
	if !ok {
		t.Fatal("no launch found")
	}
	_, arrival := projectileClosestApproach(t, velocity, target, physics, 200)
	// The steep arc that also lands 30 blocks away is airborne for about a hundred ticks.
	if arrival > 30 {
		t.Fatalf("arrived after %d ticks; want the flat arc", arrival)
	}
	if velocity[1] <= 0 || velocity[1] > 0.5 {
		t.Fatalf("vertical launch speed = %v, want the flat arc's small lift", velocity[1])
	}
}

func TestProjectileLaunchToward_RejectsUnreachableTargets(t *testing.T) {
	physics := ProjectilePhysics{Speed: 1.5, Gravity: 0.025}
	for name, tc := range map[string]struct {
		target   mgl64.Vec3
		physics  ProjectilePhysics
		maxTicks int
	}{
		"launch point":      {mgl64.Vec3{}, physics, 200},
		"out of range":      {mgl64.Vec3{0, 0, 200}, physics, 200},
		"too high":          {mgl64.Vec3{0, 60, 0}, physics, 200},
		"beyond tick limit": {mgl64.Vec3{0, 0, 20}, physics, 5},
		"no speed":          {mgl64.Vec3{0, 0, 5}, ProjectilePhysics{Gravity: 0.025}, 200},
		"full drag":         {mgl64.Vec3{0, 0, 5}, ProjectilePhysics{Speed: 1.5, Drag: 1}, 200},
	} {
		if velocity, ok := ProjectileLaunchToward(tc.target, tc.physics, tc.maxTicks); ok {
			t.Errorf("%s: launch = %v, want none", name, velocity)
		}
	}
}
