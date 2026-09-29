package entity

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// PredictEnderPearlLanding predicts the first block hit of a newly thrown pearl.
// The caller supplies the initial velocity and per-tick forces, allowing servers
// with custom pearl physics to use their own values. It returns false when the
// flight reaches unavailable terrain or the tick budget without a block hit.
// Moving entities are intentionally outside this block-only prediction.
func PredictEnderPearlLanding(src world.BlockSource, available func(cube.Pos) bool, start, velocity mgl64.Vec3, gravity, drag float64, maxTicks int) (mgl64.Vec3, bool) {
	if src == nil || available == nil || maxTicks <= 0 || gravity < 0 || drag < 0 || drag >= 1 {
		return mgl64.Vec3{}, false
	}
	for _, v := range []mgl64.Vec3{start, velocity} {
		for _, component := range v {
			if math.IsNaN(component) || math.IsInf(component, 0) {
				return mgl64.Vec3{}, false
			}
		}
	}
	pos := start
	for range maxTicks {
		// ProjectileBehaviour applies drag before gravity to Y, and drag to X/Z.
		velocity = velocity.Mul(1 - drag)
		velocity[1] -= gravity
		end := pos.Add(velocity)
		if mgl64.FloatEqual(end.Sub(pos).LenSqr(), 0) {
			return mgl64.Vec3{}, false
		}
		var hit mgl64.Vec3
		found, known := false, true
		trace.TraverseBlocks(pos, end, func(cell cube.Pos) bool {
			if !available(cell) {
				known = false
				return false
			}
			if result, ok := trace.BlockIntercept(cell, src, src.Block(cell), pos, end); ok {
				hit, found = result.Position(), true
				return false
			}
			return true
		})
		if !known {
			return mgl64.Vec3{}, false
		}
		if found {
			return hit, true
		}
		pos = end
	}
	return mgl64.Vec3{}, false
}
