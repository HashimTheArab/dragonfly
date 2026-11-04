package trace

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

// ClipBlockRayStart moves an out-of-bounds segment start to its first in-bounds
// height. It never extends the segment; end stays unchanged and may leave bounds.
// The upper boundary is rounded inward so TraverseBlocks starts in a valid cell.
// Callers must still stop traversal at unavailable or out-of-bounds cells, reject
// zero-length segments, and use the original endpoints for model intersections.
func ClipBlockRayStart(bounds cube.Range, start, end mgl64.Vec3) (mgl64.Vec3, bool) {
	lower, upper := float64(bounds.Min()), float64(bounds.Max())+1
	if lower >= upper || !finiteVectors(start, end) {
		return mgl64.Vec3{}, false
	}
	if start[1] >= lower && start[1] < upper {
		return start, true
	}
	delta := end.Sub(start)
	if delta[1] == 0 || !finiteVectors(delta) {
		return mgl64.Vec3{}, false
	}
	y := lower
	if start[1] >= upper {
		y = math.Nextafter(upper, lower)
	}
	t := (y - start[1]) / delta[1]
	if t < 0 || t > 1 {
		return mgl64.Vec3{}, false
	}
	clipped := start.Add(delta.Mul(t))
	clipped[1] = y
	return clipped, ValidBlockRay(bounds, clipped, end)
}
