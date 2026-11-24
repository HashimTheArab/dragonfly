package block

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// PlacementPlan is a reachable click, the facing that aims at it, and the
// ordered writes the click produces through the block's canonical placement
// behaviour. Offset is relative to Clicked; both use wire precision.
type PlacementPlan struct {
	Clicked  cube.Pos
	Face     cube.Face
	Offset   mgl64.Vec3
	Rotation cube.Rotation
	Changes  []PredictedBlockChange
}

// PlacementPlanOptions supplies reach, actor state and caller policy for a plan.
type PlacementPlanOptions struct {
	Reach  float64
	Actors []world.PlacementActor
	// Faces limits clicked support faces. Nil tries all faces in the normal
	// support-search order. An empty non-nil slice allows no placement.
	Faces []cube.Face
	// Expected returns the exact primary block allowed at each written cell.
	// Nil permits only the requested target. It may restrict a multi-block
	// placement to a caller's selected structure or permitted search area.
	Expected func(cube.Pos) (world.Block, bool)
	// RequireYawInvariant additionally requires the same expected states at
	// each quarter-turn of the aimed yaw.
	RequireYawInvariant bool
	// Accept admits otherwise valid writes against caller-owned reservations.
	// It must not change the source or mutate the supplied changes.
	Accept func([]PredictedBlockChange) bool
}

// PlanPlacement finds a support click that places required at target using
// material when the player at pose turns to face it. Placement is predicted with
// that facing, so directional states match what the server receives. Every
// primary write must match Expected and occupy known terrain, compared through
// world.MatchesBuiltState against the terrain the click leaves: the click
// chooses neither a block's derived state, such as a stair's corner, nor its
// ambient state, such as a crop's growth. Writes may merge
// into or update existing blocks, such as completing a double slab, when the
// block's own placement behaviour does so. Reach is checked from the eye to the
// clicked block's centre and to each placed cell's centre, and actor obstruction
// at each canonical PlaceBlock call, preserving the block's
// item.UseContext.IgnoreBBox behaviour.
func PlanPlacement(src PlacementSource, registry world.BlockRegistry, pose PlacementPose, target cube.Pos, required, material world.Block, opts PlacementPlanOptions) (PlacementPlan, bool) {
	if material == nil {
		return PlacementPlan{}, false
	}
	faces := opts.Faces
	if faces == nil {
		faces = make([]cube.Face, 0, len(cube.Faces()))
		for _, side := range cube.Faces() {
			faces = append(faces, side.Opposite())
		}
	}
	return planClick(src, registry, pose, target, required, material, faces, opts, func(face cube.Face) cube.Pos { return target.Side(face.Opposite()) })
}

// PlanItemUse finds a click on target itself that turns it into required when
// the player at pose turns to face it and uses used on it, such as a hoe
// tilling dirt. It applies PlanPlacement's reach, visibility, terrain and
// matching rules. opts.Faces limits the faces of target that may be clicked;
// nil tries the top face first. Only a predictable use can be planned.
func PlanItemUse(src PlacementSource, registry world.BlockRegistry, pose PlacementPose, target cube.Pos, required world.Block, used item.PlannableUse, opts PlacementPlanOptions) (PlacementPlan, bool) {
	if used == nil {
		return PlacementPlan{}, false
	}
	faces := opts.Faces
	if faces == nil {
		faces = []cube.Face{cube.FaceUp, cube.FaceNorth, cube.FaceSouth, cube.FaceWest, cube.FaceEast, cube.FaceDown}
	}
	return planClick(src, registry, pose, target, required, used, faces, opts, func(cube.Face) cube.Pos { return target })
}

// planClick searches faces for a reachable, visible click on the block
// clicked returns for each face, predicting what using use there writes.
func planClick(src PlacementSource, registry world.BlockRegistry, pose PlacementPose, target cube.Pos, required world.Block, use any, faces []cube.Face, opts PlacementPlanOptions, clickedFor func(cube.Face) cube.Pos) (PlacementPlan, bool) {
	if src == nil || registry == nil || required == nil || opts.Reach <= 0 || math.IsInf(opts.Reach, 0) || math.IsNaN(opts.Reach) {
		return PlacementPlan{}, false
	}
	if target.OutOfBounds(src.Range()) {
		return PlacementPlan{}, false
	}
	if _, ok := placementRuntimeID(registry, required); !ok {
		return PlacementPlan{}, false
	}
	current, loaded := src.BlockLoaded(target)
	if !loaded || current == nil {
		return PlacementPlan{}, false
	}
	// A target that already counts as built under the rule its writes are
	// matched by needs no click.
	if world.MatchesBuiltState(registry, src, target, required, current) {
		return PlacementPlan{}, false
	}
	eye := pose.Position.Add(mgl64.Vec3{0, pose.EyeHeight, 0})
	limits := placementLimits{reach: opts.Reach, actors: opts.Actors}
	match := placementMatch{src: src, registry: registry, target: target, required: required, expected: opts.Expected}
	for _, face := range faces {
		if face < cube.FaceDown || face > cube.FaceEast {
			continue
		}
		clicked := clickedFor(face)
		if clicked.OutOfBounds(src.Range()) {
			continue
		}
		support, loaded := src.BlockLoaded(clicked)
		if !loaded || !CanClickToPlace(support) || !placementWithinReach(eye, clicked.Vec3Centre(), opts.Reach) {
			continue
		}
		for _, sample := range placementClickOffsets(face) {
			rotation, offset, ok := placementAim(src, eye, opts.Reach, clicked, face, support, sample)
			if !ok {
				continue
			}
			aimed := pose
			aimed.Rotation = rotation
			changes, known := predictPlacementWithLimits(src, &aimed, clicked, face, offset, use, limits)
			if !known || !match.changes(changes) {
				continue
			}
			if opts.RequireYawInvariant && !placementYawInvariant(match, aimed, clicked, face, offset, use, limits, opts.Accept) {
				continue
			}
			if opts.Accept != nil && !opts.Accept(changes) {
				continue
			}
			return PlacementPlan{Clicked: clicked, Face: face, Offset: offset, Rotation: rotation, Changes: changes}, true
		}
	}
	return PlacementPlan{}, false
}

// placementAim turns toward sample on clicked's face at wire precision and
// returns the support model hit that facing produces, so the sent facing,
// click offset and predicted state agree.
func placementAim(src PlacementSource, eye mgl64.Vec3, reach float64, clicked cube.Pos, face cube.Face, support world.Block, sample mgl64.Vec3) (cube.Rotation, mgl64.Vec3, bool) {
	d := clicked.Vec3().Add(sample).Sub(eye)
	rotation := cube.Rotation{
		float64(float32(mgl64.RadToDeg(math.Atan2(-d[0], d[2])))),
		float64(float32(mgl64.RadToDeg(-math.Atan2(d[1], math.Hypot(d[0], d[2]))))),
	}
	end := eye.Add(rotation.Vec3().Mul(reach))
	if !trace.ValidBlockRay(src.Range(), eye, end) {
		return cube.Rotation{}, mgl64.Vec3{}, false
	}
	view := &placementVisibilitySource{PlacementSource: src, known: true}
	hit, ok := trace.BlockIntercept(clicked, view, support, eye, end)
	if !ok || !view.known || hit.Face() != face {
		return cube.Rotation{}, mgl64.Vec3{}, false
	}
	offset := hit.Position().Sub(clicked.Vec3())
	offset = mgl64.Vec3{float64(float32(offset[0])), float64(float32(offset[1])), float64(float32(offset[2]))}
	point := clicked.Vec3().Add(offset)
	if eye.Sub(point).LenSqr() > reach*reach || !placementVisible(src, eye, point, clicked) {
		return cube.Rotation{}, mgl64.Vec3{}, false
	}
	return rotation, offset, true
}

// placementWithinReach applies Player.canReach's eye-distance rule to point.
// Non-finite coordinates are never within reach.
func placementWithinReach(eye, point mgl64.Vec3, reach float64) bool {
	return eye.Sub(point).Len() <= reach
}

// placementRuntimeID returns the runtime ID of b's encoded state.
func placementRuntimeID(registry world.BlockRegistry, b world.Block) (uint32, bool) {
	name, properties := b.EncodeBlock()
	return registry.StateToRuntimeID(name, properties)
}

// placementClickOffsets returns aim targets at the face centre and both vertical
// halves for click-sensitive canonical placement rules such as slab orientation.
func placementClickOffsets(face cube.Face) []mgl64.Vec3 {
	centre := cube.Pos{}.Side(face).Vec3().Mul(0.5).Add(mgl64.Vec3{0.5, 0.5, 0.5})
	if face == cube.FaceUp || face == cube.FaceDown {
		return []mgl64.Vec3{centre}
	}
	lower, upper := centre, centre
	lower[1], upper[1] = 0.25, 0.75
	return []mgl64.Vec3{centre, lower, upper}
}

// placementVisibilitySource distinguishes unknown terrain from authentic air
// while the trace visits block models and their neighbouring cells.
type placementVisibilitySource struct {
	PlacementSource
	known bool
}

// Block reads only loaded terrain and remembers any unavailable query.
func (s *placementVisibilitySource) Block(pos cube.Pos) world.Block {
	b, loaded := s.BlockLoaded(pos)
	if !loaded || b == nil {
		s.known = false
		return Air{}
	}
	return b
}

// placementVisible requires model visibility and known terrain along the trace.
func placementVisible(src PlacementSource, eye, hit mgl64.Vec3, clicked cube.Pos) bool {
	view := &placementVisibilitySource{PlacementSource: src, known: true}
	return trace.BlockVisibility(view, src.Range(), eye, hit, clicked) == trace.VisibilityClear && view.known
}

// placementMatch validates predicted writes against one PlanPlacement request.
type placementMatch struct {
	src      PlacementSource
	registry world.BlockRegistry
	target   cube.Pos
	required world.Block
	expected func(cube.Pos) (world.Block, bool)
}

// changes validates all layers and matches every primary write against the
// requested target and the caller's expected state.
func (m placementMatch) changes(changes []PredictedBlockChange) bool {
	after := &placementView{src: m.src, changes: changes, known: true}
	matched := false
	for _, change := range changes {
		if change.Pos.OutOfBounds(m.src.Range()) || change.Block == nil {
			return false
		}
		if _, loaded := m.src.BlockLoaded(change.Pos); !loaded {
			return false
		}
		if _, valid := NormaliseLayerBlock(change.Block, change.Layer); !valid {
			return false
		}
		if change.Layer != 0 {
			continue
		}
		if _, known := placementRuntimeID(m.registry, change.Block); !known {
			return false
		}
		if change.Pos == m.target {
			// Expected may cover neighbouring cells, but it cannot relax the
			// requested target state.
			if !world.MatchesBuiltState(m.registry, after, change.Pos, m.required, change.Block) {
				return false
			}
			matched = true
		}
		if m.expected == nil {
			if change.Pos != m.target {
				return false
			}
			continue
		}
		wanted, found := m.expected(change.Pos)
		if !found || wanted == nil || !world.MatchesBuiltState(m.registry, after, change.Pos, wanted, change.Block) {
			return false
		}
	}
	// A derived state read through unloaded terrain is not known to match.
	return matched && after.known
}

// placementYawInvariant validates the requested writes at every quarter-turn.
func placementYawInvariant(match placementMatch, pose PlacementPose, clicked cube.Pos, face cube.Face, offset mgl64.Vec3, use any, limits placementLimits, accept func([]PredictedBlockChange) bool) bool {
	for quarterTurn := 1; quarterTurn < 4; quarterTurn++ {
		turned := pose
		turned.Rotation[0] += float64(quarterTurn * 90)
		changes, known := predictPlacementWithLimits(match.src, &turned, clicked, face, offset, use, limits)
		if !known || !match.changes(changes) || accept != nil && !accept(changes) {
			return false
		}
	}
	return true
}
