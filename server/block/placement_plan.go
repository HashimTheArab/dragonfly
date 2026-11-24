package block

import (
	"math"
	"slices"

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
	// FreeAim lets a plan, when no aimed click works, click any face of a
	// loaded support within reach, such as the top of a wall above the eye or
	// a face behind other blocks, and face any horizontal direction while
	// doing so, so directional blocks such as stairs take the facing they need.
	// The clicked block must still be a real support: servers refuse a click
	// on air, but check only its reach.
	FreeAim bool
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
	return planSupportClick(src, registry, pose, target, required, material, opts)
}

// PlanUsePlacement finds a support click that places required at target by
// using used on the support beside it, such as a water bucket emptied against
// a neighbouring block. It applies PlanPlacement's rules, and never plans water
// in a dimension where it evaporates.
func PlanUsePlacement(src PlacementSource, registry world.BlockRegistry, pose PlacementPose, target cube.Pos, required world.Block, used item.UsableOnBlock, opts PlacementPlanOptions) (PlacementPlan, bool) {
	if used == nil || src == nil {
		return PlacementPlan{}, false
	}
	if _, water := required.(Water); water && src.Dimension().WaterEvaporates() {
		// Water poured here evaporates at once, which prediction cannot see.
		return PlacementPlan{}, false
	}
	return planSupportClick(src, registry, pose, target, required, used, opts)
}

// PlanScaffold finds a temporary block for a target no click can place yet,
// such as the first log of a sideways row with nothing along its axis: a cell
// beside target that allowed admits and opts.Expected does not, where scaffold
// can be placed now and from which a click then places required. It returns
// that cell and the plan placing scaffold there; the caller removes the
// scaffold once required is built, so a scaffold required needs to stay up,
// such as the block under sand or a torch, is never planned.
func PlanScaffold(src PlacementSource, registry world.BlockRegistry, pose PlacementPose, target cube.Pos, required, material, scaffold world.Block, allowed func(cube.Pos) bool, opts PlacementPlanOptions) (cube.Pos, PlacementPlan, bool) {
	if src == nil || material == nil || scaffold == nil {
		return cube.Pos{}, PlacementPlan{}, false
	}
	for _, face := range cube.Faces() {
		cell := target.Side(face)
		if cell.OutOfBounds(src.Range()) || allowed != nil && !allowed(cell) {
			continue
		}
		if current, loaded := src.BlockLoaded(cell); !loaded || current == nil || !world.CanReplace(current, scaffold) {
			continue
		}
		if scaffoldFalls(src, cell, scaffold) {
			// A falling scaffold could leave before the click it supports.
			continue
		}
		if opts.Expected != nil {
			if _, inside := opts.Expected(cell); inside {
				continue
			}
		}
		// The caller's limits on faces and writes are for required, not the scaffold.
		scaffoldOpts := opts
		scaffoldOpts.Expected, scaffoldOpts.RequireYawInvariant, scaffoldOpts.Faces, scaffoldOpts.Accept = nil, false, nil, nil
		plan, ok := planSupportClick(src, registry, pose, cell, scaffold, scaffold, scaffoldOpts)
		if !ok {
			continue
		}
		after := &placementView{src: src, changes: plan.Changes, known: true}
		placed, ok := planSupportClick(after, registry, pose, target, required, material, opts)
		if ok && standsWithout(src, registry, slices.Concat(plan.Changes, placed.Changes), target, required, cell) {
			return cell, plan, true
		}
	}
	return cube.Pos{}, PlacementPlan{}, false
}

// scaffoldFalls reports whether scaffold would fall from cell, by the rule a
// falling block uses, or whether the terrain below is not known.
func scaffoldFalls(src PlacementSource, cell cube.Pos, scaffold world.Block) (falls bool) {
	if !IsGravityAffected(scaffold) {
		return false
	}
	view := &placementView{src: src, known: true}
	world.RunBlockTransaction(view, func(tx *world.Tx) { falls = replaceableWith(tx, cell.Side(cube.FaceDown), scaffold) })
	return falls || !view.known
}

// standsWithout reports whether the block changes leave at target still is
// required when removed is emptied, running its neighbour update against the
// result: an update may rewrite the block, such as a fence dropping the arm it
// had towards removed, as long as the result still matches. A block whose
// update cannot be predicted does not count as standing.
func standsWithout(src PlacementSource, registry world.BlockRegistry, changes []PredictedBlockChange, target cube.Pos, required world.Block, removed cube.Pos) (stands bool) {
	view := &placementView{src: src, changes: append(changes, PredictedBlockChange{Pos: removed, Block: Air{}}), known: true}
	ticker, ok := view.Block(target).(world.NeighbourUpdateTicker)
	if !ok {
		return view.known
	}
	written := len(view.changes)
	defer func() {
		if recover() != nil {
			// A break or fall reaches the world past the block writes.
			stands = false
		}
	}()
	world.RunBlockTransaction(view, func(tx *world.Tx) { ticker.NeighbourUpdateTick(target, removed, tx) })
	for _, change := range view.changes[written:] {
		if change.Pos == target && (change.Layer != 0 || !world.MatchesBuiltState(registry, view, target, required, change.Block)) {
			return false
		}
	}
	return view.known
}

// planSupportClick plans a click on a support beside target, trying aimed
// clicks before free-aim ones when opts allows them.
func planSupportClick(src PlacementSource, registry world.BlockRegistry, pose PlacementPose, target cube.Pos, required world.Block, use any, opts PlacementPlanOptions) (PlacementPlan, bool) {
	faces := opts.Faces
	if faces == nil {
		faces = make([]cube.Face, 0, len(cube.Faces()))
		for _, side := range cube.Faces() {
			faces = append(faces, side.Opposite())
		}
	}
	eye := pose.Position.Add(mgl64.Vec3{0, pose.EyeHeight, 0})
	clicked := func(face cube.Face) cube.Pos { return target.Side(face.Opposite()) }
	var memo clickMemo
	if plan, ok := planClick(src, registry, pose, target, required, use, faces, opts, clicked, supportAim(src, eye, opts.Reach, false), false, &memo); ok || !opts.FreeAim || !memo.freeAimMayDiffer(src, faces, clicked, eye, opts.Reach) {
		return plan, ok
	}
	return planClick(src, registry, pose, target, required, use, faces, opts, clicked, supportAim(src, eye, opts.Reach, true), true, &memo)
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
	eye := pose.Position.Add(mgl64.Vec3{0, pose.EyeHeight, 0})
	clicked := func(cube.Face) cube.Pos { return target }
	var memo clickMemo
	if plan, ok := planClick(src, registry, pose, target, required, used, faces, opts, clicked, supportAim(src, eye, opts.Reach, false), false, &memo); ok || !opts.FreeAim || !memo.freeAimMayDiffer(src, faces, clicked, eye, opts.Reach) {
		return plan, ok
	}
	return planClick(src, registry, pose, target, required, used, faces, opts, clicked, supportAim(src, eye, opts.Reach, true), true, &memo)
}

// clickAim admits a face of clicked and returns how to aim at sample points
// on it: the facing that reaches each point and the click offset it produces.
type clickAim func(clicked cube.Pos, face cube.Face) (func(sample mgl64.Vec3) (cube.Rotation, mgl64.Vec3, bool), bool)

// supportAim aims at the model of a loaded block a click may place against.
// A blind aim turns straight to the sample point without requiring the look to
// reach it first through the terrain.
func supportAim(src PlacementSource, eye mgl64.Vec3, reach float64, blind bool) clickAim {
	return func(clicked cube.Pos, face cube.Face) (func(mgl64.Vec3) (cube.Rotation, mgl64.Vec3, bool), bool) {
		support, loaded := src.BlockLoaded(clicked)
		if !loaded || !CanClickToPlace(support) {
			return nil, false
		}
		if !blind {
			return func(sample mgl64.Vec3) (cube.Rotation, mgl64.Vec3, bool) {
				return placementAim(src, eye, reach, clicked, face, support, sample)
			}, true
		}
		return func(sample mgl64.Vec3) (cube.Rotation, mgl64.Vec3, bool) {
			point := clicked.Vec3().Add(sample)
			if eye.Sub(point).LenSqr() > reach*reach {
				return cube.Rotation{}, mgl64.Vec3{}, false
			}
			d := point.Sub(eye)
			rotation := cube.Rotation{
				float64(float32(mgl64.RadToDeg(math.Atan2(-d[0], d[2])))),
				float64(float32(mgl64.RadToDeg(-math.Atan2(d[1], math.Hypot(d[0], d[2]))))),
			}
			return rotation, placementWireOffset(sample), true
		}, true
	}
}

// planRotations lists the facings a click is tried with: the aimed one, and
// with freeYaw also each horizontal direction at the aimed pitch. It returns
// how many of the five are used.
func planRotations(aimed cube.Rotation, freeYaw bool) ([5]cube.Rotation, int) {
	if !freeYaw {
		return [5]cube.Rotation{aimed}, 1
	}
	return [5]cube.Rotation{aimed, {0, aimed[1]}, {90, aimed[1]}, {180, aimed[1]}, {-90, aimed[1]}}, 5
}

// clickMemo records, for each face and click sample an aimed pass predicted,
// whether that placement read the user's facing. A free-aim pass then skips
// clicks whose writes another facing cannot change.
type clickMemo [6][3]clickTried

// clickTried is what an aimed pass learnt about one click.
type clickTried uint8

const (
	clickUntried clickTried = iota
	// clickFacingFree was predicted without reading the user's facing.
	clickFacingFree
	// clickFacingRead was predicted and read the user's facing.
	clickFacingRead
)

// freeAimMayDiffer reports whether a free-aim pass could find a click the
// aimed pass recorded in m did not: a clickable face in reach with a click
// sample the aimed pass never predicted, or one whose placement read the
// user's facing.
func (m *clickMemo) freeAimMayDiffer(src PlacementSource, faces []cube.Face, clickedFor func(cube.Face) cube.Pos, eye mgl64.Vec3, reach float64) bool {
	if src == nil {
		return false
	}
	for _, face := range faces {
		if face < cube.FaceDown || face > cube.FaceEast {
			continue
		}
		clicked := clickedFor(face)
		if clicked.OutOfBounds(src.Range()) || !placementWithinReach(eye, clicked.Vec3Centre(), reach) {
			continue
		}
		if support, loaded := src.BlockLoaded(clicked); !loaded || !CanClickToPlace(support) {
			continue
		}
		_, samples := placementClickOffsets(face)
		for i := range samples {
			if m[face][i] != clickFacingFree {
				return true
			}
		}
	}
	return false
}

// planClick searches faces for a reachable, visible click on the block
// clicked returns for each face, aimed through aim, predicting what using use
// there writes. An aimed pass records into memo what a freeYaw pass reads.
func planClick(src PlacementSource, registry world.BlockRegistry, pose PlacementPose, target cube.Pos, required world.Block, use any, faces []cube.Face, opts PlacementPlanOptions, clickedFor func(cube.Face) cube.Pos, aim clickAim, freeYaw bool, memo *clickMemo) (PlacementPlan, bool) {
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
		if !placementWithinReach(eye, clicked.Vec3Centre(), opts.Reach) {
			continue
		}
		aimAt, ok := aim(clicked, face)
		if !ok {
			continue
		}
		offsets, samples := placementClickOffsets(face)
		for i, sample := range offsets[:samples] {
			tried := &memo[face][i]
			if freeYaw && *tried == clickFacingFree {
				// The aimed pass predicted this click, and no facing changes it.
				continue
			}
			aimedRotation, offset, ok := aimAt(sample)
			if !ok {
				continue
			}
			if !freeYaw && placementNearSample(offset, sample) {
				// A hit a rounding off the sample clicks the sample itself:
				// the very click a free-aim pass sends, so its prediction
				// stands for that click.
				offset = placementWireOffset(sample)
			}
			rotations, facings := planRotations(aimedRotation, freeYaw)
			for j, rotation := range rotations[:facings] {
				if j == 0 && freeYaw && *tried == clickFacingRead {
					// The aimed pass already predicted the aimed facing.
					continue
				}
				aimed := pose
				aimed.Rotation = rotation
				changes, known, rotated := predictPlacementWithLimits(src, &aimed, clicked, face, offset, use, limits)
				if !freeYaw && placementSameSpot(offset, sample) {
					// Only a click on the very spot a free-aim pass clicks
					// stands for it; one the model took elsewhere does not.
					*tried = clickFacingFree
					if rotated {
						*tried = clickFacingRead
					}
				}
				if !known || !match.changes(changes) {
					if !rotated {
						// Every other facing writes the same.
						break
					}
					continue
				}
				if opts.RequireYawInvariant && rotated && !placementYawInvariant(match, aimed, clicked, face, offset, use, limits, opts.Accept) {
					continue
				}
				if opts.Accept != nil && !opts.Accept(changes) {
					continue
				}
				return PlacementPlan{Clicked: clicked, Face: face, Offset: offset, Rotation: rotation, Changes: changes}, true
			}
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
	offset = placementWireOffset(offset)
	point := clicked.Vec3().Add(offset)
	if eye.Sub(point).LenSqr() > reach*reach || !placementVisible(src, eye, point, clicked) {
		return cube.Rotation{}, mgl64.Vec3{}, false
	}
	return rotation, offset, true
}

// placementSameSpot reports whether an aimed click sends the very offset a
// click on sample sends. Only the same wire offset stands for it: a block
// such as a slab picks its half from the offset, so even a rounding apart can
// place differently.
func placementSameSpot(offset, sample mgl64.Vec3) bool {
	return placementWireOffset(offset) == placementWireOffset(sample)
}

// placementNearSample reports whether an aimed hit landed on sample within
// the rounding of the aim's float32 rotation.
func placementNearSample(offset, sample mgl64.Vec3) bool {
	const epsilon = 1e-4
	d := offset.Sub(sample)
	return math.Abs(d[0]) < epsilon && math.Abs(d[1]) < epsilon && math.Abs(d[2]) < epsilon
}

// placementWireOffset rounds a click offset to the precision the wire sends.
func placementWireOffset(offset mgl64.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{float64(float32(offset[0])), float64(float32(offset[1])), float64(float32(offset[2]))}
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
// halves for click-sensitive canonical placement rules such as slab
// orientation, and how many of the three a face uses.
func placementClickOffsets(face cube.Face) ([3]mgl64.Vec3, int) {
	centre := cube.Pos{}.Side(face).Vec3().Mul(0.5).Add(mgl64.Vec3{0.5, 0.5, 0.5})
	if face == cube.FaceUp || face == cube.FaceDown {
		return [3]mgl64.Vec3{centre}, 1
	}
	lower, upper := centre, centre
	lower[1], upper[1] = 0.25, 0.75
	return [3]mgl64.Vec3{centre, lower, upper}, 3
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
		if change.Pos == m.target {
			// Expected may cover neighbouring cells, but it cannot relax the
			// requested target state. A wrong state fails here, before the
			// registry check below encodes it.
			if !world.MatchesBuiltState(m.registry, after, change.Pos, m.required, change.Block) {
				return false
			}
			matched = true
		}
		if _, known := placementRuntimeID(m.registry, change.Block); !known {
			return false
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
		changes, known, _ := predictPlacementWithLimits(match.src, &turned, clicked, face, offset, use, limits)
		if !known || !match.changes(changes) || accept != nil && !accept(changes) {
			return false
		}
	}
	return true
}
