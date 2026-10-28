package block

import (
	"math"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func TestBlock_PlanPlacementReachAndTerrain(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	target := cube.Pos{0, 64, 0}
	user := PlacementPose{Position: mgl64.Vec3{0.5, 64, 2.5}, EyeHeight: 1.62}
	cases := []struct {
		name   string
		edit   func(*placementTestSource, *PlacementPlanOptions)
		placed bool
	}{
		{name: "reachable", placed: true},
		{name: "occupied target", edit: func(src *placementTestSource, _ *PlacementPlanOptions) { src.blocks[target] = Stone{} }},
		{name: "unknown target", edit: func(src *placementTestSource, _ *PlacementPlanOptions) { src.unknown[target] = true }},
		{name: "unknown support", edit: func(src *placementTestSource, _ *PlacementPlanOptions) {
			src.unknown[target.Side(cube.FaceDown)] = true
		}},
		{name: "no support", edit: func(src *placementTestSource, _ *PlacementPlanOptions) {
			delete(src.blocks, target.Side(cube.FaceDown))
		}},
		{name: "interactive support", edit: func(src *placementTestSource, _ *PlacementPlanOptions) {
			src.blocks[target.Side(cube.FaceDown)] = NewChest()
		}},
		{name: "replaceable support", edit: func(src *placementTestSource, _ *PlacementPlanOptions) {
			src.blocks[target.Side(cube.FaceDown)] = ShortGrass{}
		}},
		{name: "blocked view", edit: func(src *placementTestSource, _ *PlacementPlanOptions) { src.blocks[cube.Pos{0, 64, 1}] = Stone{} }},
		{name: "unknown view", edit: func(src *placementTestSource, _ *PlacementPlanOptions) { src.unknown[cube.Pos{0, 64, 1}] = true }},
		{name: "too far", edit: func(_ *placementTestSource, opts *PlacementPlanOptions) { opts.Reach = 1 }},
		{name: "invalid reach", edit: func(_ *placementTestSource, opts *PlacementPlanOptions) { opts.Reach = math.Inf(1) }},
		{name: "caller reservation", edit: func(_ *placementTestSource, opts *PlacementPlanOptions) {
			opts.Accept = func([]PredictedBlockChange) bool { return false }
		}},
		{name: "no permitted face", edit: func(_ *placementTestSource, opts *PlacementPlanOptions) { opts.Faces = []cube.Face{} }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{target.Side(cube.FaceDown): Stone{}})
			opts := PlacementPlanOptions{Reach: 5, Faces: []cube.Face{cube.FaceUp}}
			if test.edit != nil {
				test.edit(src, &opts)
			}
			plan, placed := PlanPlacement(src, world.DefaultBlockRegistry, user, target, Stone{}, Stone{}, opts)
			if placed != test.placed {
				t.Fatalf("placed = %v, want %v; plan=%#v", placed, test.placed, plan)
			}
			if placed {
				if plan.Clicked != target.Side(cube.FaceDown) || plan.Face != cube.FaceUp {
					t.Fatalf("click = %v/%v, want support below/up", plan.Clicked, plan.Face)
				}
				assertPlacementChanges(t, plan.Changes, PredictedBlockChange{Pos: target, Block: Stone{}})
			}
		})
	}
}

func TestBlock_PlanPlacementRejectsNonFiniteEye(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	target := cube.Pos{0, 64, 0}
	src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{target.Side(cube.FaceDown): Stone{}})
	user := PlacementPose{Position: mgl64.Vec3{math.NaN(), 64, 2.5}, EyeHeight: 1.62}
	if _, placed := PlanPlacement(src, world.DefaultBlockRegistry, user, target, Stone{}, Stone{}, PlacementPlanOptions{Reach: 5}); placed {
		t.Fatal("planned from a non-finite eye")
	}
}

func TestBlock_PlanPlacementExactStatesAndYaw(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	target := cube.Pos{0, 64, 0}
	user := PlacementPose{Position: mgl64.Vec3{2.5, 64, 2.5}, EyeHeight: 1.62, Rotation: cube.Rotation{0, 0}}
	opts := PlacementPlanOptions{Reach: 5, Faces: []cube.Face{cube.FaceEast}}
	src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{target.Side(cube.FaceWest): Stone{}})
	for _, top := range []bool{false, true} {
		t.Run(map[bool]string{false: "bottom slab", true: "top slab"}[top], func(t *testing.T) {
			required := Slab{Block: Stone{}, Top: top}
			plan, placed := PlanPlacement(src, world.DefaultBlockRegistry, user, target, required, Slab{Block: Stone{}}, opts)
			if !placed {
				t.Fatal("no exact slab plan")
			}
			assertPlacementChanges(t, plan.Changes, PredictedBlockChange{Pos: target, Block: required})
		})
	}
	if _, placed := PlanPlacement(src, world.DefaultBlockRegistry, user, target, Cobblestone{}, Stone{}, opts); placed {
		t.Fatal("accepted a material producing the wrong state")
	}
	opts.Faces = []cube.Face{cube.FaceUp}
	src = newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{target.Side(cube.FaceDown): Stone{}})
	// The aim toward the support's top face from the north-east sets the stair facing.
	aimed := cube.Rotation{135, 0}.Direction()
	stairs := Stairs{Block: Stone{}, Facing: aimed}
	plan, placed := PlanPlacement(src, world.DefaultBlockRegistry, user, target, stairs, Stairs{Block: Stone{}}, opts)
	if !placed || plan.Rotation.Direction() != aimed {
		t.Fatalf("no aimed-yaw stair plan: %v %v", placed, plan.Rotation)
	}
	if _, placed := PlanPlacement(src, world.DefaultBlockRegistry, user, target, Stairs{Block: Stone{}, Facing: aimed.Opposite()}, Stairs{Block: Stone{}}, opts); placed {
		t.Fatal("accepted a stair facing away from the aim")
	}
	opts.RequireYawInvariant = true
	if _, placed := PlanPlacement(src, world.DefaultBlockRegistry, user, target, stairs, Stairs{Block: Stone{}}, opts); placed {
		t.Fatal("accepted a stair whose state changes with yaw")
	}
	if _, placed := PlanPlacement(src, world.DefaultBlockRegistry, user, target, Stone{}, Stone{}, opts); !placed {
		t.Fatal("rejected yaw-invariant stone")
	}
}

func TestBlock_PlanPlacementActorRules(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	target := cube.Pos{0, 64, 0}
	user := PlacementPose{Position: mgl64.Vec3{0.5, 64, 2.5}, EyeHeight: 1.62}
	for _, test := range []struct {
		name   string
		typ    world.EntityType
		placed bool
	}{
		{name: "solid", typ: placementTestEntityType{name: "minecraft:player"}},
		{name: "unregistered"},
		{name: "passable", typ: placementTestPassableType{placementTestEntityType{name: "minecraft:item"}}, placed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{target.Side(cube.FaceDown): Stone{}})
			opts := PlacementPlanOptions{
				Reach:  5,
				Actors: []world.PlacementActor{{Type: test.typ, Bounds: cube.Box(0.1, 64, 0.1, 0.9, 65.8, 0.9)}},
			}
			if _, placed := PlanPlacement(src, world.DefaultBlockRegistry, user, target, Stone{}, Stone{}, opts); placed != test.placed {
				t.Fatalf("placed = %v, want %v", placed, test.placed)
			}
		})
	}
}

func TestBlock_PlanPlacementPreservesIgnoreBBox(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	target := cube.Pos{0, 64, 0}
	user := PlacementPose{Position: mgl64.Vec3{0.5, 64, -2.5}, EyeHeight: 1.62, Rotation: cube.Rotation{0, 0}}
	bed := Bed{Colour: item.ColourWhite(), Facing: user.Rotation.Direction()}
	headPos := target.Side(bed.Facing.Face())
	head := bed
	head.Head = true
	src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{
		target.Side(cube.FaceDown):  Stone{},
		headPos.Side(cube.FaceDown): Stone{},
	})
	opts := PlacementPlanOptions{
		Reach: 5, Faces: []cube.Face{cube.FaceUp},
		Actors: []world.PlacementActor{{Type: placementTestEntityType{name: "minecraft:player"}, Bounds: cube.Box(0.1, 64, 0.1, 0.9, 65.8, 0.9)}},
		Expected: func(pos cube.Pos) (world.Block, bool) {
			switch pos {
			case target:
				return bed, true
			case headPos:
				return head, true
			}
			return nil, false
		},
	}
	plan, placed := PlanPlacement(src, world.DefaultBlockRegistry, user, target, bed, bed, opts)
	if !placed {
		t.Fatal("canonical bed IgnoreBBox was lost")
	}
	assertPlacementChanges(t, plan.Changes, PredictedBlockChange{Pos: headPos, Block: head}, PredictedBlockChange{Pos: target, Block: bed})
}

func TestBlock_PlanPlacementRejectsPartialMultiBlock(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	target := cube.Pos{0, 64, 0}
	upper := target.Side(cube.FaceUp)
	user := PlacementPose{Position: mgl64.Vec3{0.5, 64, 2.5}, EyeHeight: 1.62}
	for _, test := range []struct {
		name   string
		edit   func(*placementTestSource, *PlacementPlanOptions)
		placed bool
	}{
		{name: "both cells allowed", placed: true},
		{name: "missing upper expectation", edit: func(_ *placementTestSource, opts *PlacementPlanOptions) {
			opts.Expected = func(pos cube.Pos) (world.Block, bool) { return Stone{}, pos == target }
		}},
		{name: "unloaded upper", edit: func(src *placementTestSource, _ *PlacementPlanOptions) { src.unknown[upper] = true }},
		{name: "actor blocks only upper", edit: func(_ *placementTestSource, opts *PlacementPlanOptions) {
			opts.Actors = []world.PlacementActor{{Type: placementTestEntityType{name: "minecraft:player"}, Bounds: cube.Box(0.1, 65.1, 0.1, 0.9, 66.8, 0.9)}}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{target.Side(cube.FaceDown): Stone{}})
			opts := PlacementPlanOptions{
				Reach: 5, Faces: []cube.Face{cube.FaceUp},
				Expected: func(pos cube.Pos) (world.Block, bool) { return Stone{}, pos == target || pos == upper },
			}
			if test.edit != nil {
				test.edit(src, &opts)
			}
			plan, placed := PlanPlacement(src, world.DefaultBlockRegistry, user, target, Stone{}, placementTestTwoBlocks{}, opts)
			if placed != test.placed {
				t.Fatalf("placed = %v, want %v; changes=%#v", placed, test.placed, plan.Changes)
			}
			if placed {
				assertPlacementChanges(t, plan.Changes, PredictedBlockChange{Pos: target, Block: Stone{}}, PredictedBlockChange{Pos: upper, Block: Stone{}})
			}
		})
	}
}

// placementTestTwoBlocks exercises an item whose second PlaceBlock call can
// fail independently of the first, with solid collision boxes in both cells.
type placementTestTwoBlocks struct{ Stone }

// UseOnBlock places two solid cells through the canonical user's admission.
func (placementTestTwoBlocks) UseOnBlock(pos cube.Pos, face cube.Face, _ mgl64.Vec3, _ *world.Tx, user item.User, ctx *item.UseContext) bool {
	target := pos.Side(face)
	placer := user.(Placer)
	placer.PlaceBlock(target, Stone{}, ctx)
	placer.PlaceBlock(target.Side(cube.FaceUp), Stone{}, ctx)
	return ctx.CountSub > 0
}

func TestBlock_NormaliseLayerBlock(t *testing.T) {
	for _, test := range []struct {
		name  string
		block world.Block
		layer uint8
		want  world.Block
		valid bool
	}{
		{name: "nil primary", want: Air{}, valid: true},
		{name: "solid primary", block: Stone{}, want: Stone{}, valid: true},
		{name: "nil secondary", layer: 1, want: Air{}, valid: true},
		{name: "air secondary", block: Air{}, layer: 1, want: Air{}, valid: true},
		{name: "water secondary", block: Water{Depth: 8}, layer: 1, want: Water{Depth: 8}, valid: true},
		{name: "lava secondary", block: Lava{Depth: 8}, layer: 1, want: Lava{Depth: 8}, valid: true},
		{name: "solid secondary", block: Stone{}, layer: 1},
		{name: "unknown layer", block: Stone{}, layer: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, valid := NormaliseLayerBlock(test.block, test.layer)
			if got != test.want || valid != test.valid {
				t.Fatalf("normalised = %#v, %v; want %#v, %v", got, valid, test.want, test.valid)
			}
		})
	}
}

func TestBlock_PlanPlacementRotationAimsAtClick(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	target := cube.Pos{0, 64, 0}
	clicked := target.Side(cube.FaceWest)
	src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{clicked: Stone{}})
	user := PlacementPose{Position: mgl64.Vec3{2.3, 64, .7}, EyeHeight: 1.62}
	eye := user.Position.Add(mgl64.Vec3{0, user.EyeHeight, 0})
	for _, slab := range []Slab{{Block: Stone{}}, {Block: Stone{}, Top: true}} {
		plan, ok := PlanPlacement(src, world.DefaultBlockRegistry, user, target, slab, Slab{Block: Stone{}}, PlacementPlanOptions{Reach: 5, Faces: []cube.Face{cube.FaceEast}})
		if !ok {
			t.Fatalf("no plan for %#v", slab)
		}
		for i, v := range append(plan.Offset[:], plan.Rotation[:]...) {
			if v != float64(float32(v)) {
				t.Fatalf("value %d=%v is not wire precision", i, v)
			}
		}
		hit, ok := trace.BlockIntercept(clicked, src, Stone{}, eye, eye.Add(plan.Rotation.Vec3().Mul(5)))
		if !ok || hit.Face() != cube.FaceEast || hit.Position().Sub(clicked.Vec3().Add(plan.Offset)).Len() > 1e-5 {
			t.Fatalf("facing %v does not reach click %v", plan.Rotation, plan.Offset)
		}
		assertPlacementChanges(t, plan.Changes, PredictedBlockChange{Pos: target, Block: slab})
	}
	// An obstruction must not become a new click target for this placement.
	src.blocks[cube.Pos{1, 64, 0}] = Stone{}
	src.blocks[cube.Pos{1, 65, 0}] = Stone{}
	if _, ok := PlanPlacement(src, world.DefaultBlockRegistry, user, target, Stone{}, Stone{}, PlacementPlanOptions{Reach: 5, Faces: []cube.Face{cube.FaceEast}}); ok {
		t.Fatal("aim crossed obstructed terrain")
	}
}

func TestBlock_PlanPlacementReachUsesCellCentres(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	target := cube.Pos{0, 64, 0}
	upper := target.Side(cube.FaceUp)
	for _, test := range []struct {
		name   string
		eye    mgl64.Vec3
		placed placementTestTwoBlocks
		reach  float64
		want   bool
	}{
		// The hit point (~4.001) and target centre (~3.501) are in reach, but
		// the clicked block's centre (~4.501) is not.
		{name: "clicked centre out of reach", eye: mgl64.Vec3{0.6, 68, 0.5}, reach: 4.2},
		{name: "clicked centre in reach", eye: mgl64.Vec3{0.6, 68, 0.5}, reach: 4.6, want: true},
		// The hit point (~4.005) and clicked centre (~4.061) are in reach, but
		// the second placed cell's centre (~4.206) is not.
		{name: "later cell out of reach", eye: mgl64.Vec3{4.5, 64.2, 0.5}, reach: 4.1},
		{name: "later cell in reach", eye: mgl64.Vec3{4.5, 64.2, 0.5}, reach: 4.3, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{target.Side(cube.FaceDown): Stone{}})
			opts := PlacementPlanOptions{
				Reach: test.reach, Faces: []cube.Face{cube.FaceUp},
				Expected: func(pos cube.Pos) (world.Block, bool) { return Stone{}, pos == target || pos == upper },
			}
			plan, placed := PlanPlacement(src, world.DefaultBlockRegistry, PlacementPose{Position: test.eye}, target, Stone{}, test.placed, opts)
			if placed != test.want {
				t.Fatalf("placed = %v, want %v; changes=%#v", placed, test.want, plan.Changes)
			}
		})
	}
}

func TestBlock_PlanPlacementMergesExistingBlocks(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()
	target := cube.Pos{0, 64, 0}
	user := PlacementPose{Position: mgl64.Vec3{0.5, 64, 2.5}, EyeHeight: 1.62}
	half, double := Slab{Block: Stone{}}, Slab{Block: Stone{}, Double: true}
	for _, test := range []struct {
		name     string
		current  world.Block
		required world.Block
		want     bool
	}{
		{name: "completes double slab", current: half, required: double, want: true},
		{name: "already satisfied", current: double, required: double},
		{name: "different slab type", current: Slab{Block: Cobblestone{}}, required: double},
	} {
		t.Run(test.name, func(t *testing.T) {
			// A bottom half slab covers its support's top face, so the merge
			// clicks the side of a neighbour.
			src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{
				target.Side(cube.FaceWest): Stone{},
				target:                     test.current,
			})
			plan, placed := PlanPlacement(src, world.DefaultBlockRegistry, user, target, test.required, half, PlacementPlanOptions{Reach: 5, Faces: []cube.Face{cube.FaceEast}})
			if placed != test.want {
				t.Fatalf("placed = %v, want %v; changes=%#v", placed, test.want, plan.Changes)
			}
			if placed {
				assertPlacementChanges(t, plan.Changes, PredictedBlockChange{Pos: target, Block: double})
			}
		})
	}
}
