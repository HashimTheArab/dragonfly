package block

import (
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// PlacementPlan is a reachable click and the ordered writes it produces through
// the block's canonical placement behaviour. Offset is relative to Clicked.
type PlacementPlan struct {
	Clicked cube.Pos
	Face    cube.Face
	Offset  mgl64.Vec3
	Changes []PredictedBlockChange
}

// PlacementPlanOptions supplies reach, actor state and caller policy for a plan.
// The caller must keep src and user coherent throughout PlanPlacement.
type PlacementPlanOptions struct {
	EyePosition mgl64.Vec3
	Reach       float64
	Actors      []world.PlacementActor
	// Faces limits clicked support faces. Nil tries all faces in the normal
	// support-search order. An empty non-nil slice allows no placement.
	Faces []cube.Face
	// RequireFacing restricts the click to the ray from EyePosition along
	// user.Rotation. The exact model hit supplies the click offset, so a
	// moving user need not keep aiming at a sampled face centre.
	RequireFacing bool
	// Expected returns the exact primary block allowed at each written cell.
	// Nil permits only the requested target. It may restrict a multi-block
	// placement to a caller's selected structure or permitted search area.
	Expected func(cube.Pos) (world.Block, bool)
	// RequireYawInvariant additionally requires the same expected states at
	// each quarter-turn of the player's captured yaw.
	RequireYawInvariant bool
	// Accept admits otherwise valid writes against caller-owned reservations.
	// It must not change the source or mutate the supplied changes.
	Accept func([]PredictedBlockChange) bool
}

// PlanPlacement finds a reachable support click that places required at target
// using material. Every primary write must match Expected, occupy known terrain
// and replace its current block. Visibility also requires known terrain. Actor
// obstruction is checked at each canonical PlaceBlock call, preserving the
// block's item.UseContext.IgnoreBBox behaviour.
func PlanPlacement(src PlacementSource, registry world.BlockRegistry, user PlacementUser, target cube.Pos, required, material world.Block, opts PlacementPlanOptions) (PlacementPlan, bool) {
	if src == nil || registry == nil || user == nil || required == nil || material == nil || opts.Reach <= 0 || math.IsInf(opts.Reach, 0) || math.IsNaN(opts.Reach) {
		return PlacementPlan{}, false
	}
	if target.OutOfBounds(src.Range()) {
		return PlacementPlan{}, false
	}
	current, loaded := src.BlockLoaded(target)
	if !loaded || !world.CanReplace(current, material) {
		return PlacementPlan{}, false
	}
	faces := opts.Faces
	if faces == nil {
		faces = make([]cube.Face, 0, len(cube.Faces()))
		for _, side := range cube.Faces() {
			faces = append(faces, side.Opposite())
		}
	}
	for _, face := range faces {
		if face < cube.FaceDown || face > cube.FaceEast {
			continue
		}
		clicked := target.Side(face.Opposite())
		if clicked.OutOfBounds(src.Range()) {
			continue
		}
		support, loaded := src.BlockLoaded(clicked)
		if !loaded || !CanClickToPlace(support) {
			continue
		}
		offsets := placementClickOffsets(face)
		if opts.RequireFacing {
			end := opts.EyePosition.Add(user.Rotation().Vec3().Mul(opts.Reach))
			if !trace.ValidBlockRay(src.Range(), opts.EyePosition, end) {
				return PlacementPlan{}, false
			}
			view := &placementVisibilitySource{PlacementSource: src, known: true}
			hit, ok := trace.BlockIntercept(clicked, view, support, opts.EyePosition, end)
			if !ok || !view.known || hit.Face() != face {
				continue
			}
			offsets = []mgl64.Vec3{hit.Position().Sub(clicked.Vec3())}
		}
		for _, offset := range offsets {
			hit := clicked.Vec3().Add(offset)
			if opts.EyePosition.Sub(hit).LenSqr() > opts.Reach*opts.Reach || !placementVisible(src, opts.EyePosition, hit, clicked) {
				continue
			}
			changes, known := predictPlacementWithActors(src, user, clicked, face, offset, material, opts.Actors)
			if !known || !placementChangesMatch(src, registry, changes, target, required, opts.Expected) {
				continue
			}
			if opts.RequireYawInvariant && !placementYawInvariant(src, registry, user, clicked, face, offset, material, target, required, opts) {
				continue
			}
			if opts.Accept != nil && !opts.Accept(changes) {
				continue
			}
			return PlacementPlan{Clicked: clicked, Face: face, Offset: offset, Changes: changes}, true
		}
	}
	return PlacementPlan{}, false
}

// placementClickOffsets samples the face centre and both vertical halves for
// click-sensitive canonical placement rules such as slab orientation.
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

// placementChangesMatch validates all layers and matches every primary write
// against the requested target and the caller's expected state.
func placementChangesMatch(src PlacementSource, registry world.BlockRegistry, changes []PredictedBlockChange, target cube.Pos, required world.Block, expected func(cube.Pos) (world.Block, bool)) bool {
	matched := false
	for _, change := range changes {
		if change.Pos.OutOfBounds(src.Range()) || change.Block == nil {
			return false
		}
		current, loaded := src.BlockLoaded(change.Pos)
		if !loaded {
			return false
		}
		if _, valid := NormaliseLayerBlock(change.Block, change.Layer); !valid {
			return false
		}
		if change.Layer != 0 {
			continue
		}
		wanted := required
		if expected != nil {
			var found bool
			wanted, found = expected(change.Pos)
			if !found || wanted == nil {
				return false
			}
		} else if change.Pos != target {
			return false
		}
		name, properties := wanted.EncodeBlock()
		wantedID, wantedKnown := registry.StateToRuntimeID(name, properties)
		name, properties = change.Block.EncodeBlock()
		actualID, actualKnown := registry.StateToRuntimeID(name, properties)
		if !wantedKnown || !actualKnown || wantedID != actualID || !world.CanReplace(current, change.Block) {
			return false
		}
		if change.Pos == target {
			// Expected may cover neighbouring cells, but it cannot relax the
			// requested target state.
			name, properties = required.EncodeBlock()
			requiredID, requiredKnown := registry.StateToRuntimeID(name, properties)
			if !requiredKnown || requiredID != actualID {
				return false
			}
			matched = true
		}
	}
	return matched
}

// placementTurnedUser overrides yaw while preserving the supplied user's pose.
type placementTurnedUser struct {
	PlacementUser
	rotation cube.Rotation
}

// Rotation returns the quarter-turned orientation being validated.
func (u placementTurnedUser) Rotation() cube.Rotation { return u.rotation }

// EyeHeight preserves optional eye-height input used by vertical placement rules.
func (u placementTurnedUser) EyeHeight() float64 {
	if eyed, ok := u.PlacementUser.(interface{ EyeHeight() float64 }); ok {
		return eyed.EyeHeight()
	}
	return 0
}

// placementYawInvariant validates the requested writes at every quarter-turn.
func placementYawInvariant(src PlacementSource, registry world.BlockRegistry, user PlacementUser, clicked cube.Pos, face cube.Face, offset mgl64.Vec3, material world.Block, target cube.Pos, required world.Block, opts PlacementPlanOptions) bool {
	for quarterTurn := 1; quarterTurn < 4; quarterTurn++ {
		rotation := user.Rotation()
		rotation[0] += float64(quarterTurn * 90)
		turned := placementTurnedUser{PlacementUser: user, rotation: rotation}
		changes, known := predictPlacementWithActors(src, turned, clicked, face, offset, material, opts.Actors)
		if !known || !placementChangesMatch(src, registry, changes, target, required, opts.Expected) || opts.Accept != nil && !opts.Accept(changes) {
			return false
		}
	}
	return true
}
