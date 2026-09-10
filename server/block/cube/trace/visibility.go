package trace

import (
	"math"
	"slices"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// Visibility describes a block-model visibility query.
type Visibility uint8

const (
	VisibilityInvalid Visibility = iota
	VisibilityClear
	VisibilityBlocked
)

// BlockVisibility checks a segment against block models.
// Invalid geometry is distinct from terrain obstruction, so it cannot be
// mistaken for protective cover.
// Every visited cell must lie within bounds, including ignored cells. Ignored
// blocks allow a ray to end on or start inside the block being interacted with.
// Zero-length segments inside bounds are clear. Non-finite coordinates are invalid.
func BlockVisibility(src world.BlockSource, bounds cube.Range, start, end mgl64.Vec3, ignored ...cube.Pos) Visibility {
	for _, v := range []mgl64.Vec3{start, end} {
		for _, n := range v {
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return VisibilityInvalid
			}
		}
		if y := cube.PosFromVec3(v).Y(); y < bounds.Min() || y > bounds.Max() {
			return VisibilityInvalid
		}
	}
	if mgl64.FloatEqual(end.Sub(start).LenSqr(), 0) {
		return VisibilityClear
	}
	result := VisibilityClear
	TraverseBlocks(start, end, func(pos cube.Pos) bool {
		if pos.Y() < bounds.Min() || pos.Y() > bounds.Max() {
			result = VisibilityInvalid
			return false
		}
		if slices.Contains(ignored, pos) {
			return true
		}
		if BlockIntersects(pos, src, src.Block(pos), start, end) {
			result = VisibilityBlocked
			return false
		}
		return true
	})
	return result
}
