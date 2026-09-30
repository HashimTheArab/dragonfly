package entity

import (
	"math"
	"sort"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
)

// FitProjectilePhysics estimates constant air gravity and drag from a projectile's
// observed launch velocity and ordered airborne positions. Packet intervals need
// not correspond to ticks: horizontal motion determines each sample's integer
// flight tick. The caller must exclude collisions, liquid movement, and teleports.
//
// At least five distinct samples spanning six ticks are required. Near-vertical,
// inconsistent, or ambiguous trajectories return false. maxTicks bounds the
// search and must be between 6 and 240; at most 64 positions may be supplied.
// Speed is the magnitude of the observed launch velocity, not a fitted bow charge.
// Both force/displacement phases are considered. An indistinguishable phase uses
// the BDS default; materially different compatible models return false.
func FitProjectilePhysics(start, velocity mgl64.Vec3, positions []mgl64.Vec3, maxTicks int) (ProjectilePhysics, bool) {
	if len(positions) < 5 || len(positions) > 64 || maxTicks < 6 || maxTicks > 240 {
		return ProjectilePhysics{}, false
	}
	for _, v := range append([]mgl64.Vec3{start, velocity}, positions...) {
		for _, component := range v {
			if !projectileFinite32(component) {
				return ProjectilePhysics{}, false
			}
		}
	}
	horizontal := math.Hypot(velocity[0], velocity[2])
	if horizontal < 0.1 {
		return ProjectilePhysics{}, false
	}
	direction := mgl64.Vec3{velocity[0] / horizontal, 0, velocity[2] / horizontal}
	previous := 0.0
	tolerance := 0.0
	for _, position := range positions {
		delta := position.Sub(start)
		distance := delta.Dot(direction)
		if distance <= previous {
			return ProjectilePhysics{}, false
		}
		previous = distance
		tolerance = math.Max(tolerance, projectileObservationTolerance(position))
	}

	// Tick inference makes the objective piecewise smooth. Locate each coarse
	// local minimum, then refine it within its neighbouring drag interval.
	const step = 0.001
	best := projectileFit{error: math.Inf(1)}
	var fits []projectileFit
	for _, forcesBeforeMove := range []bool{false, true} {
		candidates := make([]projectileFit, 1000)
		for i := range candidates {
			candidates[i] = fitProjectileDrag(start, velocity, positions, direction, float64(i)*step, forcesBeforeMove, maxTicks)
		}
		for i, candidate := range candidates {
			if i > 0 && candidate.error > candidates[i-1].error || i+1 < len(candidates) && candidate.error > candidates[i+1].error || math.IsInf(candidate.error, 1) {
				continue
			}
			low, high := math.Max(0, float64(i-1)*step), math.Min(0.999999, float64(i+1)*step)
			for range 28 {
				left, right := low+(high-low)/3, high-(high-low)/3
				a := fitProjectileDrag(start, velocity, positions, direction, left, forcesBeforeMove, maxTicks)
				b := fitProjectileDrag(start, velocity, positions, direction, right, forcesBeforeMove, maxTicks)
				if a.error < candidate.error {
					candidate = a
				}
				if b.error < candidate.error {
					candidate = b
				}
				if a.error <= b.error {
					high = right
				} else {
					low = left
				}
			}
			fits = append(fits, candidate)
			if candidate.error < best.error {
				best = candidate
			}
		}
	}
	if best.error > 1 {
		return ProjectilePhysics{}, false
	}
	for _, candidate := range fits {
		phaseDiffers := candidate.physics.ForcesBeforeMove != best.physics.ForcesBeforeMove &&
			math.Max(projectilePhaseEffect(velocity, candidate), projectilePhaseEffect(velocity, best)) > 2*tolerance
		if candidate.error <= 1 && (math.Abs(candidate.physics.Drag-best.physics.Drag) > 0.002 || math.Abs(candidate.physics.Gravity-best.physics.Gravity) > 0.001 || phaseDiffers) {
			return ProjectilePhysics{}, false
		}
	}
	best.physics.Speed = velocity.Len()
	if projectilePhaseEffect(velocity, best) <= tolerance {
		best.physics.ForcesBeforeMove = false
	}
	return best.physics, true
}

type projectileFit struct {
	physics  ProjectilePhysics
	error    float64
	lastTick int
}

func fitProjectileDrag(start, velocity mgl64.Vec3, positions []mgl64.Vec3, direction mgl64.Vec3, drag float64, forcesBeforeMove bool, maxTicks int) projectileFit {
	invalid := projectileFit{error: math.Inf(1)}
	pos := mgl32.Vec3{float32(start[0]), float32(start[1]), float32(start[2])}
	motion := mgl32.Vec3{float32(velocity[0]), float32(velocity[1]), float32(velocity[2])}
	inertia := float32(1 - drag)
	if forcesBeforeMove {
		motion = projectileNextVelocity(motion, inertia, 0)
	}
	distances := make([]float64, maxTicks+1)
	sums, gravitySums := make([]float64, maxTicks+1), make([]float64, maxTicks+1)
	power := 1.0
	for tick := 1; tick <= maxTicks; tick++ {
		pos = pos.Add(motion)
		motion = projectileNextVelocity(motion, inertia, 0)
		distances[tick] = projectileVec64(pos).Sub(start).Dot(direction)
		sums[tick] = sums[tick-1] + power
		gravitySums[tick] = gravitySums[tick-1] + sums[tick-1]
		power *= float64(inertia)
	}
	var ticks []int
	previous := 0
	var numerator, denominator float64
	for _, position := range positions {
		if previous >= maxTicks {
			return invalid
		}
		distance := position.Sub(start).Dot(direction)
		lo := previous + 1
		tick := lo + sort.Search(maxTicks-lo+1, func(i int) bool { return distances[lo+i] >= distance })
		if tick > maxTicks {
			tick = maxTicks
		}
		if tick > lo && math.Abs(distances[tick-1]-distance) <= math.Abs(distances[tick]-distance) {
			tick--
		}
		previous = tick
		ticks = append(ticks, tick)
		weight := gravitySums[tick]
		launchSum := sums[tick]
		if forcesBeforeMove {
			weight += sums[tick]
			launchSum *= float64(inertia)
		}
		numerator += weight * (start[1] + velocity[1]*launchSum - position[1])
		denominator += weight * weight
	}
	if ticks[len(ticks)-1]-ticks[0] < 6 || denominator == 0 {
		return invalid
	}
	gravity := math.Max(0, numerator/denominator)
	if !projectileFinite32(gravity) {
		return invalid
	}
	pos = mgl32.Vec3{float32(start[0]), float32(start[1]), float32(start[2])}
	motion = mgl32.Vec3{float32(velocity[0]), float32(velocity[1]), float32(velocity[2])}
	if forcesBeforeMove {
		motion = projectileNextVelocity(motion, inertia, float32(gravity))
	}
	var error float64
	sample := 0
	for tick := 1; tick <= ticks[len(ticks)-1]; tick++ {
		pos = pos.Add(motion)
		motion = projectileNextVelocity(motion, inertia, float32(gravity))
		if tick == ticks[sample] {
			distance := projectileVec64(pos).Sub(positions[sample]).Len()
			if !projectileFinite32(distance) {
				return invalid
			}
			tolerance := projectileObservationTolerance(positions[sample])
			error = math.Max(error, distance/tolerance)
			sample++
		}
	}
	return projectileFit{physics: ProjectilePhysics{Gravity: gravity, Drag: drag, ForcesBeforeMove: forcesBeforeMove}, error: error, lastTick: ticks[len(ticks)-1]}
}

// Switching phase shifts the displacement sum from v0..v(n-1) to v1..vn.
// Their difference telescopes to vn-v0. Ignore only changes within the position
// observation's precision, including flights with no forces at all.
func projectilePhaseEffect(velocity mgl64.Vec3, fit projectileFit) float64 {
	initial := mgl32.Vec3{float32(velocity[0]), float32(velocity[1]), float32(velocity[2])}
	motion := initial
	for range fit.lastTick {
		motion = projectileNextVelocity(motion, float32(1-fit.physics.Drag), float32(fit.physics.Gravity))
	}
	return float64(motion.Sub(initial).Len())
}

func projectileObservationTolerance(position mgl64.Vec3) float64 {
	tolerance := 0.002
	for _, component := range position {
		value := float32(math.Abs(component))
		ulp := float64(math.Nextafter32(value, float32(math.Inf(1))) - value)
		tolerance = math.Max(tolerance, 8*ulp)
	}
	return tolerance
}
