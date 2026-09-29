package entity

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// EnderPearlPrediction is the first block hit of a newly thrown pearl, or the
// last position reached before its flight leaves available terrain or its tick
// budget. Hit is false for the latter case; Block is then undefined.
type EnderPearlPrediction struct {
	Position mgl64.Vec3
	Block    cube.Pos
	Hit      bool
}

// PredictEnderPearlLanding predicts the first block hit of a newly thrown pearl.
// The caller supplies the initial velocity and per-tick forces, allowing servers
// with custom pearl physics to use their own values. When the flight reaches
// unavailable terrain or the tick budget, the last known position is returned
// without a block hit. It returns false only when the inputs are invalid or no
// complete flight step could be simulated.
// Moving entities are intentionally outside this block-only prediction.
func PredictEnderPearlLanding(src world.BlockSource, available func(cube.Pos) bool, start, velocity mgl64.Vec3, gravity, drag float64, maxTicks int) (EnderPearlPrediction, bool) {
	if src == nil || available == nil || maxTicks <= 0 || gravity < 0 || drag < 0 || drag >= 1 {
		return EnderPearlPrediction{}, false
	}
	for _, v := range []mgl64.Vec3{start, velocity} {
		for _, component := range v {
			if math.IsNaN(component) || math.IsInf(component, 0) {
				return EnderPearlPrediction{}, false
			}
		}
	}
	pos := start
	advanced := false
	for range maxTicks {
		// ProjectileBehaviour applies drag before gravity to Y, and drag to X/Z.
		velocity = velocity.Mul(1 - drag)
		velocity[1] -= gravity
		end := pos.Add(velocity)
		if mgl64.FloatEqual(end.Sub(pos).LenSqr(), 0) {
			return EnderPearlPrediction{Position: pos}, advanced
		}
		var hit trace.BlockResult
		found, known := false, true
		trace.TraverseBlocks(pos, end, func(cell cube.Pos) bool {
			if !available(cell) {
				known = false
				return false
			}
			if result, ok := trace.BlockIntercept(cell, src, src.Block(cell), pos, end); ok {
				hit, found = result, true
				return false
			}
			return true
		})
		if !known {
			return EnderPearlPrediction{Position: pos}, advanced
		}
		if found {
			return EnderPearlPrediction{Position: hit.Position(), Block: hit.BlockPosition(), Hit: true}, true
		}
		pos = end
		advanced = true
	}
	return EnderPearlPrediction{Position: pos}, advanced
}
