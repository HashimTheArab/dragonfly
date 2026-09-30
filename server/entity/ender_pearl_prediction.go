package entity

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
)

const pearlRadiansToIndex float32 = 10430.3779296875

// EnderPearlLaunchVelocity returns the Bedrock projectile launch velocity for
// rotation and speed. Bedrock uses indexed, single-precision trigonometry;
// Rotation.Vec3 uses continuous, double-precision trigonometry instead.
// Values outside finite single precision return a non-finite velocity that
// PredictEnderPearlLanding rejects.
func EnderPearlLaunchVelocity(rotation cube.Rotation, speed float64) mgl64.Vec3 {
	const (
		degreesToRadians float32 = -0.01745329238474369
		negativePi       float32 = -3.1415927410125732
		quarterTurn      float32 = 16384
	)
	for _, value := range []float64{rotation.Yaw(), rotation.Pitch(), speed} {
		if !pearlFinite32(value) {
			return mgl64.Vec3{math.NaN(), math.NaN(), math.NaN()}
		}
	}
	yawRadians := float32(float32(rotation.Yaw()) * degreesToRadians)
	pitchRadians := float32(float32(rotation.Pitch()) * degreesToRadians)
	yawIndex := float32(float32(yawRadians+negativePi) * pearlRadiansToIndex)
	pitchIndex := float32(pitchRadians * pearlRadiansToIndex)
	cosPitch := pearlIndexedSin(pitchIndex + quarterTurn)
	direction := mgl32.Vec3{
		pearlIndexedSin(yawIndex) * -cosPitch,
		pearlIndexedSin(pitchIndex),
		pearlIndexedSin(yawIndex+quarterTurn) * -cosPitch,
	}
	// Keep each product and sum at single precision, including the reciprocal
	// multiply used by Bedrock's projectile normalization.
	xy := float32(float32(direction[0]*direction[0]) + float32(direction[1]*direction[1]))
	lengthSquared := float32(xy + float32(direction[2]*direction[2]))
	length := float32(math.Sqrt(float64(lengthSquared)))
	if length < 0.001 {
		return mgl64.Vec3{}
	}
	return pearlVec64(direction.Mul(1 / length).Mul(float32(speed)))
}

func pearlIndexedSin(index float32) float32 {
	// The Bedrock sine table stores sinf(float32(i)/pearlRadiansToIndex).
	// Evaluate only the requested entry instead of retaining a global table.
	angle := float32(uint32(int32(index))&65535) / pearlRadiansToIndex
	return float32(math.Sin(float64(angle)))
}

func pearlVec64(v mgl32.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{float64(v[0]), float64(v[1]), float64(v[2])}
}

func pearlFinite32(value float64) bool {
	return math.Abs(value) <= math.MaxFloat32
}

// EnderPearlPrediction is the first block hit of a newly thrown pearl, or the
// last position reached before its flight leaves available terrain or its tick
// budget. Hit is false for the latter case; Block and Face are then undefined.
type EnderPearlPrediction struct {
	Position mgl64.Vec3
	Block    cube.Pos
	Face     cube.Face
	Hit      bool
}

// PredictEnderPearlLanding predicts the first block hit of a newly thrown pearl.
// The caller supplies the initial velocity and per-tick forces, allowing servers
// with custom pearl physics to use their own values. When the flight reaches
// unavailable terrain or the tick budget, the last known position is returned
// without a block hit. It returns false only when the inputs are invalid or no
// complete flight step could be simulated.
// Movement uses the current velocity before drag and gravity are applied for
// the following tick, matching Bedrock Dedicated Server projectiles.
// Position, velocity, and per-tick forces use Bedrock's single precision.
// Moving entities are intentionally outside this block-only prediction.
func PredictEnderPearlLanding(src world.BlockSource, available func(cube.Pos) bool, start, velocity mgl64.Vec3, gravity, drag float64, maxTicks int) (EnderPearlPrediction, bool) {
	if src == nil || available == nil || maxTicks <= 0 || !pearlFinite32(gravity) || !pearlFinite32(drag) || gravity < 0 || drag < 0 || drag >= 1 {
		return EnderPearlPrediction{}, false
	}
	for _, v := range []mgl64.Vec3{start, velocity} {
		for _, component := range v {
			if !pearlFinite32(component) {
				return EnderPearlPrediction{}, false
			}
		}
	}
	pos := mgl32.Vec3{float32(start[0]), float32(start[1]), float32(start[2])}
	motion := mgl32.Vec3{float32(velocity[0]), float32(velocity[1]), float32(velocity[2])}
	inertia, gravity32 := float32(1-drag), float32(gravity)
	advanced := false
	for range maxTicks {
		end := pos.Add(motion)
		from, to := pearlVec64(pos), pearlVec64(end)
		var hit trace.BlockResult
		found, known := false, true
		if mgl64.FloatEqual(to.Sub(from).LenSqr(), 0) {
			// A vertical throw may have a stationary tick at its apex. Do not
			// stop its flight before gravity starts its descent.
			known = available(cube.PosFromVec3(from))
		} else {
			trace.TraverseBlocks(from, to, func(cell cube.Pos) bool {
				if !available(cell) {
					known = false
					return false
				}
				if result, ok := trace.BlockIntercept(cell, src, src.Block(cell), from, to); ok {
					hit, found = result, true
					return false
				}
				return true
			})
		}
		if !known {
			return EnderPearlPrediction{Position: from}, advanced
		}
		if found {
			point := hit.Position()
			return EnderPearlPrediction{Position: mgl64.Vec3{float64(float32(point[0])), float64(float32(point[1])), float64(float32(point[2]))}, Block: hit.BlockPosition(), Face: hit.Face(), Hit: true}, true
		}
		pos = end
		advanced = true
		for axis := range motion {
			motion[axis] = float32(motion[axis] * inertia)
		}
		motion[1] -= gravity32
	}
	return EnderPearlPrediction{Position: pearlVec64(pos)}, advanced
}
