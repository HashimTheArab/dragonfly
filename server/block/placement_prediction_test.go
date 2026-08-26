package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func TestBlock_PredictPlacementCanonicalBehaviour(t *testing.T) {
	clicked := cube.Pos{4, 64, 7}
	user := placementTestUser{pos: clicked.Vec3Centre(), rot: cube.Rotation{90, 0}}

	t.Run("slab merge", func(t *testing.T) {
		slab := Slab{Block: Stone{}}
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{clicked: slab})
		changes := PredictPlacement(src, user, clicked, cube.FaceUp, mgl64.Vec3{0.5, 1, 0.5}, slab)
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
		if changes := PredictPlacement(src, user, clicked, cube.FaceUp, mgl64.Vec3{}, DeadBush{}); len(changes) != 0 {
			t.Fatalf("changes = %v, want no placement on stone", changes)
		}
		src.blocks[clicked] = Sand{}
		if changes := PredictPlacement(src, user, clicked, cube.FaceUp, mgl64.Vec3{}, DeadBush{}); len(changes) != 1 {
			t.Fatalf("changes = %v, want placement on sand", changes)
		}
	})

	t.Run("layered liquid", func(t *testing.T) {
		placePos := clicked.Side(cube.FaceUp)
		src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{clicked: Sand{}})
		src.liquids[placePos] = Water{Still: true, Depth: 8}
		changes := PredictPlacement(src, user, clicked, cube.FaceUp, mgl64.Vec3{}, Kelp{})
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
		changes := PredictPlacement(src, user, clicked, cube.FaceUp, mgl64.Vec3{}, Wall{Block: Cobblestone{}})
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
		changes := PredictPlacement(src, user, clicked, cube.FaceUp, mgl64.Vec3{}, NewChest())
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
		changes := PredictPlacement(src, user, clicked, cube.FaceUp, mgl64.Vec3{}, Skull{Type: SkeletonSkull()})
		if len(changes) != 1 {
			t.Fatalf("changes = %v, want one", changes)
		}
		skull, ok := changes[0].Block.(Skull)
		if !ok || skull.Attach.Uint8() != StandingAttachment(user.rot.Orientation()).Uint8() {
			t.Fatalf("change = %#v, want standing orientation %v", changes[0], user.rot.Orientation())
		}
	})

	t.Run("wet sponge in nether", func(t *testing.T) {
		src := newPlacementTestSource(world.Nether, map[cube.Pos]world.Block{clicked: Stone{}})
		changes := PredictPlacement(src, user, clicked, cube.FaceUp, mgl64.Vec3{}, Sponge{Wet: true})
		if len(changes) != 1 {
			t.Fatalf("changes = %v, want one", changes)
		}
		sponge, ok := changes[0].Block.(Sponge)
		if !ok || sponge.Wet {
			t.Fatalf("change = %#v, want dry sponge", changes[0])
		}
	})
}

type placementTestSource struct {
	dim     world.Dimension
	blocks  map[cube.Pos]world.Block
	liquids map[cube.Pos]world.Liquid
}

func newPlacementTestSource(dim world.Dimension, blocks map[cube.Pos]world.Block) *placementTestSource {
	return &placementTestSource{dim: dim, blocks: blocks, liquids: make(map[cube.Pos]world.Liquid)}
}

func (s *placementTestSource) Range() cube.Range          { return s.dim.Range() }
func (s *placementTestSource) Dimension() world.Dimension { return s.dim }

func (s *placementTestSource) Block(pos cube.Pos) world.Block {
	if b, ok := s.blocks[pos]; ok {
		return b
	}
	return Air{}
}

func (s *placementTestSource) Liquid(pos cube.Pos) (world.Liquid, bool) {
	liquid, ok := s.liquids[pos]
	return liquid, ok
}

type placementTestUser struct {
	pos mgl64.Vec3
	rot cube.Rotation
}

func (u placementTestUser) Position() mgl64.Vec3    { return u.pos }
func (u placementTestUser) Rotation() cube.Rotation { return u.rot }
