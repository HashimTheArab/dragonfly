package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func TestBlock_PredictPlacementCanonicalBehaviour(t *testing.T) {
	clicked := cube.Pos{4, 64, 7}
	user := placementTestUser{pos: clicked.Vec3Centre(), rot: cube.Rotation{90, 0}}

	t.Run("slab merge", func(t *testing.T) {
		slab := Slab{Block: Stone{}}
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{clicked: slab})
		changes := predictPlacement(t, src, user, clicked, cube.FaceUp, mgl64.Vec3{0.5, 1, 0.5}, slab)
		if len(changes) != 1 {
			t.Fatalf("changes = %v, want one", changes)
		}
		got, ok := changes[0].Block.(Slab)
		if !ok || !got.Double || changes[0].Pos != clicked {
			t.Fatalf("change = %#v, want a double slab at %v", changes[0], clicked)
		}
	})

	t.Run("dead bush substrate", func(t *testing.T) {
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{clicked: Stone{}})
		if changes := predictPlacement(t, src, user, clicked, cube.FaceUp, mgl64.Vec3{}, DeadBush{}); len(changes) != 0 {
			t.Fatalf("changes = %v, want no placement on stone", changes)
		}
		src.blocks[clicked] = Sand{}
		if changes := predictPlacement(t, src, user, clicked, cube.FaceUp, mgl64.Vec3{}, DeadBush{}); len(changes) != 1 {
			t.Fatalf("changes = %v, want placement on sand", changes)
		}
	})

	t.Run("layered liquid", func(t *testing.T) {
		placePos := clicked.Side(cube.FaceUp)
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{clicked: Sand{}})
		src.liquids[placePos] = Water{Still: true, Depth: 8}
		changes := predictPlacement(t, src, user, clicked, cube.FaceUp, mgl64.Vec3{}, Kelp{})
		if len(changes) != 1 {
			t.Fatalf("changes = %v, want one", changes)
		}
		kelp, ok := changes[0].Block.(Kelp)
		if !ok || kelp.Age < 0 || kelp.Age > 24 || changes[0].Pos != placePos {
			t.Fatalf("change = %#v, want kelp aged 0-24 at %v", changes[0], placePos)
		}
	})

	t.Run("wall connections", func(t *testing.T) {
		placePos := clicked.Side(cube.FaceUp)
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{
			clicked:                      Stone{},
			placePos.Side(cube.FaceEast): Stone{},
		})
		changes := predictPlacement(t, src, user, clicked, cube.FaceUp, mgl64.Vec3{}, Wall{Block: Cobblestone{}})
		if len(changes) != 1 {
			t.Fatalf("changes = %v, want one", changes)
		}
		wall, ok := changes[0].Block.(Wall)
		if !ok || wall.EastConnection == NoWallConnection() {
			t.Fatalf("change = %#v, want east-connected wall", changes[0])
		}
	})

	t.Run("chest pairing", func(t *testing.T) {
		placePos := clicked.Side(cube.FaceUp)
		pairPos := placePos.Side(cube.FaceNorth)
		pair := NewChest()
		pair.Facing = cube.East
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{
			clicked: Stone{},
			pairPos: pair,
		})
		changes := predictPlacement(t, src, user, clicked, cube.FaceUp, mgl64.Vec3{}, NewChest())
		if len(changes) != 2 {
			t.Fatalf("changes = %v, want paired chest writes", changes)
		}
		placed, placedOK := changes[0].Block.(Chest)
		paired, pairedOK := changes[1].Block.(Chest)
		if !placedOK || !pairedOK || !placed.paired || !paired.paired || changes[1].Pos != pairPos {
			t.Fatalf("changes = %#v, want both halves paired", changes)
		}
	})

	t.Run("standing skull rotation", func(t *testing.T) {
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{clicked: Stone{}})
		changes := predictPlacement(t, src, user, clicked, cube.FaceUp, mgl64.Vec3{}, Skull{Type: SkeletonSkull()})
		if len(changes) != 1 {
			t.Fatalf("changes = %v, want one", changes)
		}
		skull, ok := changes[0].Block.(Skull)
		if !ok || skull.Attach.Uint8() != StandingAttachment(user.rot.Orientation()).Uint8() {
			t.Fatalf("change = %#v, want standing orientation %v", changes[0], user.rot.Orientation())
		}
	})

	t.Run("plain block on solid face", func(t *testing.T) {
		placePos := clicked.Side(cube.FaceUp)
		for _, b := range []world.Block{Cobblestone{}, Dirt{}, Stone{}, Planks{}, Obsidian{}} {
			src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{clicked: Stone{}})
			changes := predictPlacement(t, src, user, clicked, cube.FaceUp, mgl64.Vec3{0.5, 1, 0.5}, b)
			if len(changes) != 1 {
				t.Fatalf("%T: changes = %v, want one", b, changes)
			}
			if changes[0].Pos != placePos || changes[0].Block != b {
				t.Fatalf("%T: change = %#v, want %#v at %v", b, changes[0], b, placePos)
			}
		}
	})

	t.Run("plain block replaces clicked block", func(t *testing.T) {
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{clicked: Air{}})
		changes := predictPlacement(t, src, user, clicked, cube.FaceUp, mgl64.Vec3{}, Cobblestone{})
		if len(changes) != 1 || changes[0].Pos != clicked {
			t.Fatalf("changes = %#v, want cobblestone written at the clicked air block %v", changes, clicked)
		}
	})

	t.Run("plain block needs a replaceable target", func(t *testing.T) {
		placePos := clicked.Side(cube.FaceUp)
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{
			clicked:  Stone{},
			placePos: Stone{},
		})
		if changes := predictPlacement(t, src, user, clicked, cube.FaceUp, mgl64.Vec3{}, Cobblestone{}); len(changes) != 0 {
			t.Fatalf("changes = %v, want no placement into an occupied position", changes)
		}
	})

	t.Run("plain block outside world range", func(t *testing.T) {
		top := cube.Pos{clicked[0], world.Overworld.Range().Max(), clicked[2]}
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{top: Stone{}})
		if changes := predictPlacement(t, src, user, top, cube.FaceUp, mgl64.Vec3{}, Cobblestone{}); len(changes) != 0 {
			t.Fatalf("changes = %v, want no placement above the build limit", changes)
		}
	})

	t.Run("unknown placement target", func(t *testing.T) {
		placePos := clicked.Side(cube.FaceUp)
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{clicked: Stone{}})
		src.unknown[placePos] = true
		if changes, known := PredictPlacement(src, user, clicked, cube.FaceUp, mgl64.Vec3{}, Cobblestone{}); known {
			t.Fatalf("prediction was known with an unloaded target: changes=%v", changes)
		}
	})

	t.Run("wet sponge in nether", func(t *testing.T) {
		src := newPlacementTestSource(world.Nether, map[cube.Pos]world.Block{clicked: Stone{}})
		changes := predictPlacement(t, src, user, clicked, cube.FaceUp, mgl64.Vec3{}, Sponge{Wet: true})
		if len(changes) != 1 {
			t.Fatalf("changes = %v, want one", changes)
		}
		sponge, ok := changes[0].Block.(Sponge)
		if !ok || sponge.Wet {
			t.Fatalf("change = %#v, want dry sponge", changes[0])
		}
	})
}

func TestBlock_PlacementViewMirrorsLiquidDisplacement(t *testing.T) {
	pos := cube.Pos{4, 64, 7}
	sourceWater := Water{Still: true, Depth: 8}
	stoneSlab := Slab{Block: Stone{}}

	t.Run("primary liquid moves to layer one", func(t *testing.T) {
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{pos: sourceWater})
		view := newPlacementViewForTest(src)
		view.SetBlock(pos, stoneSlab, nil)

		if got := view.Block(pos); got != stoneSlab {
			t.Fatalf("primary block = %#v, want %#v", got, stoneSlab)
		}
		if got, ok := view.Liquid(pos); !ok || got != sourceWater {
			t.Fatalf("liquid = %#v, %v; want displaced source water", got, ok)
		}
		assertPlacementChanges(t, view.changes,
			PredictedBlockChange{Pos: pos, Block: sourceWater, Layer: 1},
			PredictedBlockChange{Pos: pos, Block: stoneSlab},
		)
	})

	t.Run("solid replacement clears layer one", func(t *testing.T) {
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{pos: stoneSlab})
		src.liquids[pos] = sourceWater
		view := newPlacementViewForTest(src)
		view.SetBlock(pos, Stone{}, nil)

		if got, ok := view.Liquid(pos); ok {
			t.Fatalf("liquid = %#v, %v; want cleared", got, ok)
		}
		assertPlacementChanges(t, view.changes,
			PredictedBlockChange{Pos: pos, Block: Air{}, Layer: 1},
			PredictedBlockChange{Pos: pos, Block: Stone{}},
		)
	})

	t.Run("later writes observe earlier layer changes", func(t *testing.T) {
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{pos: sourceWater})
		view := newPlacementViewForTest(src)
		view.SetBlock(pos, stoneSlab, nil)
		view.SetBlock(pos, Stone{}, nil)

		if got := view.Block(pos); got != (Stone{}) {
			t.Fatalf("primary block = %#v, want stone", got)
		}
		if got, ok := view.Liquid(pos); ok {
			t.Fatalf("liquid = %#v, %v; want cleared", got, ok)
		}
		assertPlacementChanges(t, view.changes,
			PredictedBlockChange{Pos: pos, Block: sourceWater, Layer: 1},
			PredictedBlockChange{Pos: pos, Block: stoneSlab},
			PredictedBlockChange{Pos: pos, Block: Air{}, Layer: 1},
			PredictedBlockChange{Pos: pos, Block: Stone{}},
		)
	})

	t.Run("removing displacer restores liquid to primary", func(t *testing.T) {
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{pos: stoneSlab})
		src.liquids[pos] = sourceWater
		view := newPlacementViewForTest(src)
		view.SetBlock(pos, Air{}, nil)

		if got := view.Block(pos); got != sourceWater {
			t.Fatalf("primary block = %#v, want restored source water", got)
		}
		assertPlacementChanges(t, view.changes,
			PredictedBlockChange{Pos: pos, Block: Air{}, Layer: 1},
			PredictedBlockChange{Pos: pos, Block: sourceWater},
		)
	})
}

func TestBlock_PlacementViewSetLiquidUsesVanillaLayer(t *testing.T) {
	pos := cube.Pos{4, 64, 7}
	sourceWater := Water{Still: true, Depth: 8}

	t.Run("air receives primary liquid", func(t *testing.T) {
		view := newPlacementViewForTest(newPlacementTestSource(world.Overworld, nil))
		view.SetLiquid(pos, sourceWater)
		if got := view.Block(pos); got != sourceWater {
			t.Fatalf("primary block = %#v, want source water", got)
		}
		assertPlacementChanges(t, view.changes, PredictedBlockChange{Pos: pos, Block: sourceWater})
	})

	t.Run("replaceable plant retains primary and receives layer one", func(t *testing.T) {
		grass := ShortGrass{}
		view := newPlacementViewForTest(newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{pos: grass}))
		view.SetLiquid(pos, sourceWater)
		if got := view.Block(pos); got != grass {
			t.Fatalf("primary block = %#v, want short grass", got)
		}
		assertPlacementChanges(t, view.changes, PredictedBlockChange{Pos: pos, Block: sourceWater, Layer: 1})
	})

	t.Run("non-displacing solid rejects liquid", func(t *testing.T) {
		view := newPlacementViewForTest(newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{pos: Stone{}}))
		view.SetLiquid(pos, sourceWater)
		if len(view.changes) != 0 {
			t.Fatalf("changes = %#v, want none", view.changes)
		}
	})
}

func TestBlock_PlacementViewHonoursDisabledLiquidDisplacement(t *testing.T) {
	t.Parallel()

	pos := cube.Pos{4, 64, 7}
	water := Water{Still: true, Depth: 8}
	view := newPlacementViewForTest(newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{pos: Slab{Block: Stone{}}}))
	view.src.(*placementTestSource).liquids[pos] = water
	if !world.RunBlockTransaction(view, func(tx *world.Tx) {
		tx.SetBlock(pos, Stone{}, &world.SetOpts{DisableLiquidDisplacement: true})
	}) {
		t.Fatal("supported block write was marked incomplete")
	}
	if got, ok := view.Liquid(pos); !ok || got != water {
		t.Fatalf("liquid = %#v, %v; want preserved secondary water", got, ok)
	}
	assertPlacementChanges(t, view.changes, PredictedBlockChange{Pos: pos, Block: Stone{}})
}

func TestBlock_PlacementViewRejectsWritesOutsideWorldRange(t *testing.T) {
	t.Parallel()

	view := newPlacementViewForTest(newPlacementTestSource(world.Overworld, nil))
	pos := cube.Pos{4, world.Overworld.Range().Max() + 1, 7}
	view.SetBlock(pos, Stone{}, nil)
	view.SetLiquid(pos, Water{Still: true, Depth: 8})
	if len(view.changes) != 0 {
		t.Fatalf("changes = %#v, want no writes outside the world range", view.changes)
	}
}

func TestBlock_PlacementUserUnsupportedOperationsMarkUnknown(t *testing.T) {
	t.Parallel()

	view := newPlacementViewForTest(newPlacementTestSource(world.Overworld, nil))
	user := &placementUser{view: view}
	operations := []struct {
		name string
		call func()
	}{
		{name: "entity handle", call: func() { _ = user.H() }},
		{name: "close", call: func() { _ = user.Close() }},
		{name: "held items", call: func() { _, _ = user.HeldItems() }},
		{name: "set held items", call: func() { user.SetHeldItems(item.Stack{}, item.Stack{}) }},
		{name: "using item", call: func() { _ = user.UsingItem() }},
		{name: "release item", call: user.ReleaseItem},
		{name: "use item", call: user.UseItem},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			view.known = true
			func() {
				defer func() {
					if _, ok := recover().(placementUserUnsupported); !ok {
						t.Fatal("unsupported user operation did not raise the prediction sentinel")
					}
				}()
				operation.call()
			}()
			if view.known {
				t.Fatal("unsupported user operation left prediction marked known")
			}
		})
	}
}

func TestBlock_PredictPlacementUnsupportedUserAccessReturnsUnknown(t *testing.T) {
	t.Parallel()

	clicked := cube.Pos{4, 64, 7}
	src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{clicked: Stone{}})
	changes, known := PredictPlacement(src, placementTestUser{}, clicked, cube.FaceUp, mgl64.Vec3{}, placementUnsupportedUserBlock{})
	if known {
		t.Fatalf("prediction using unsupported user state was marked known: changes=%v", changes)
	}
}

type placementUnsupportedUserBlock struct{ Stone }

func (placementUnsupportedUserBlock) UseOnBlock(_ cube.Pos, _ cube.Face, _ mgl64.Vec3, _ *world.Tx, user item.User, _ *item.UseContext) bool {
	_ = user.H().Type()
	return false
}

type placementTestSource struct {
	dim     world.Dimension
	blocks  map[cube.Pos]world.Block
	liquids map[cube.Pos]world.Liquid
	unknown map[cube.Pos]bool
}

func newPlacementTestSource(dim world.Dimension, blocks map[cube.Pos]world.Block) *placementTestSource {
	return &placementTestSource{
		dim: dim, blocks: blocks,
		liquids: make(map[cube.Pos]world.Liquid),
		unknown: make(map[cube.Pos]bool),
	}
}

func newPlacementViewForTest(src PlacementSource) *placementView {
	return &placementView{
		src:   src,
		known: true,
	}
}

func assertPlacementChanges(t *testing.T, got []PredictedBlockChange, want ...PredictedBlockChange) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("changes = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i].Pos != want[i].Pos || got[i].Block != want[i].Block || got[i].Layer != want[i].Layer {
			t.Fatalf("change %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func (s *placementTestSource) Range() cube.Range          { return s.dim.Range() }
func (s *placementTestSource) Dimension() world.Dimension { return s.dim }

func (s *placementTestSource) Block(pos cube.Pos) world.Block {
	if b, ok := s.blocks[pos]; ok {
		return b
	}
	return Air{}
}

func (s *placementTestSource) BlockLoaded(pos cube.Pos) (world.Block, bool) {
	if s.unknown[pos] {
		return nil, false
	}
	return s.Block(pos), true
}

func (s *placementTestSource) Liquid(pos cube.Pos) (world.Liquid, bool) {
	if liquid, ok := s.blocks[pos].(world.Liquid); ok {
		return liquid, true
	}
	liquid, ok := s.liquids[pos]
	return liquid, ok
}

type placementTestUser struct {
	pos mgl64.Vec3
	rot cube.Rotation
}

func (u placementTestUser) Position() mgl64.Vec3    { return u.pos }
func (u placementTestUser) Rotation() cube.Rotation { return u.rot }

func predictPlacement(t *testing.T, src PlacementSource, user PlacementUser, clickedPos cube.Pos, face cube.Face, clickPos mgl64.Vec3, placed world.Block) []PredictedBlockChange {
	t.Helper()
	changes, known := PredictPlacement(src, user, clickedPos, face, clickPos, placed)
	if !known {
		t.Fatal("prediction unexpectedly depended on unknown world state")
	}
	return changes
}
