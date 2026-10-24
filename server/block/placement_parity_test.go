package block

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"testing"
)

func TestPredictPlacementSubstrateAndAttachment(t *testing.T) {
	origin := cube.Pos{0, 64, 0}
	for _, tt := range []struct {
		name    string
		placed  world.Block
		support world.Block
		face    cube.Face
		extra   map[cube.Pos]world.Block
		want    bool
	}{
		{name: "kelp on magma", placed: Kelp{}, support: Magma{}, face: cube.FaceUp, extra: map[cube.Pos]world.Block{origin.Side(cube.FaceUp): Water{Depth: 8, Still: true}}},
		{name: "kelp on fence", placed: Kelp{}, support: WoodFence{}, face: cube.FaceUp, extra: map[cube.Pos]world.Block{origin.Side(cube.FaceUp): Water{Depth: 8, Still: true}}},
		{name: "kelp on stone", placed: Kelp{}, support: Stone{}, face: cube.FaceUp, extra: map[cube.Pos]world.Block{origin.Side(cube.FaceUp): Water{Depth: 8, Still: true}}, want: true},
		{name: "pickle on pot", placed: SeaPickle{}, support: DecoratedPot{}, face: cube.FaceUp, want: true},
		{name: "torch below ceiling with side support", placed: Torch{}, support: Stone{}, face: cube.FaceDown, extra: map[cube.Pos]world.Block{origin.Side(cube.FaceDown).Side(cube.FaceEast): Stone{}}, want: true},
		{name: "torch below unsupported ceiling", placed: Torch{}, support: Stone{}, face: cube.FaceDown},
	} {
		t.Run(tt.name, func(t *testing.T) {
			src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{origin: tt.support})
			for pos, b := range tt.extra {
				src.blocks[pos] = b
			}
			changes := predictPlacement(t, src, nil, origin, tt.face, mgl64.Vec3{}, tt.placed)
			if got := len(changes) > 0; got != tt.want {
				t.Fatalf("placement=%v, want %v; changes=%#v", got, tt.want, changes)
			}
			for _, c := range changes {
				if torch, ok := c.Block.(Torch); ok && torch.Facing != cube.FaceEast {
					t.Fatalf("torch faces %v, want east support", torch.Facing)
				}
			}
		})
	}
}

func TestPredictDoorPlacementAndHinge(t *testing.T) {
	pos := cube.Pos{0, 64, 0}
	user := placementTestUser{rot: cube.Rotation{0, 0}}
	facing := user.Rotation().Direction()
	for _, door := range []world.Block{WoodDoor{Wood: SpruceWood()}, CopperDoor{Waxed: true}, IronDoor{}} {
		for _, tt := range []struct {
			name        string
			clicked     world.Block
			face        cube.Face
			left, right int
			wantRight   bool
		}{
			{name: "replace grass from side", clicked: ShortGrass{}, face: cube.FaceNorth},
			{name: "upper right support", clicked: ShortGrass{}, face: cube.FaceUp, right: 2, wantRight: true},
			{name: "equal upper supports", clicked: ShortGrass{}, face: cube.FaceUp, left: 2, right: 2},
			{name: "left outweighs right", clicked: ShortGrass{}, face: cube.FaceUp, left: 3, right: 1},
		} {
			name, _ := door.EncodeBlock()
			t.Run(tt.name+"/"+name, func(t *testing.T) {
				src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{pos: tt.clicked, pos.Side(cube.FaceDown): Stone{}})
				for level := 0; level < 2; level++ {
					if tt.left&(1<<level) != 0 {
						src.blocks[pos.Side(facing.RotateLeft().Face()).Add(cube.Pos{0, level, 0})] = Stone{}
					}
					if tt.right&(1<<level) != 0 {
						src.blocks[pos.Side(facing.RotateRight().Face()).Add(cube.Pos{0, level, 0})] = Stone{}
					}
				}
				changes := predictPlacement(t, src, user, pos, tt.face, mgl64.Vec3{}, door)
				if len(changes) != 2 || changes[0].Pos != pos || changes[1].Pos != pos.Side(cube.FaceUp) {
					t.Fatalf("door changes=%#v", changes)
				}
				for _, c := range changes {
					var right bool
					switch d := c.Block.(type) {
					case WoodDoor:
						right = d.Right
					case CopperDoor:
						right = d.Right
					case IronDoor:
						right = d.Right
					}
					if right != tt.wantRight {
						t.Fatalf("hinge right=%v, want %v", right, tt.wantRight)
					}
				}
			})
		}
	}
}

type eyedPlacementTestUser struct {
	placementTestUser
	height float64
}

// EyeHeight supplies the player's current eye offset to placement prediction.
func (u eyedPlacementTestUser) EyeHeight() float64 { return u.height }

func TestPredictPlacementPreservesEyeHeight(t *testing.T) {
	pos := cube.Pos{0, 64, 0}
	src := newPlacementTestSource(world.Overworld, map[cube.Pos]world.Block{pos.Side(cube.FaceDown): Stone{}})
	user := eyedPlacementTestUser{placementTestUser: placementTestUser{pos: mgl64.Vec3{0.5, 65, 0.5}}, height: 1.62}
	changes := predictPlacement(t, src, user, pos.Side(cube.FaceDown), cube.FaceUp, mgl64.Vec3{}, Barrel{})
	if len(changes) != 1 {
		t.Fatalf("changes=%v", changes)
	}
	if got := changes[0].Block.(Barrel).Facing; got != cube.FaceUp {
		t.Fatalf("barrel facing=%v, want up", got)
	}
}

func TestKelpLosesInvalidSupport(t *testing.T) {
	w := world.Config{Synchronous: true}.New()
	defer w.Close()
	pos := cube.Pos{0, 64, 0}
	runWorld(w, func(tx *world.Tx) {
		tx.SetBlock(pos.Side(cube.FaceDown), Stone{}, nil)
		tx.SetBlock(pos, Kelp{}, nil)
		tx.SetLiquid(pos, Water{Depth: 8, Still: true})
		tx.SetBlock(pos.Side(cube.FaceDown), Magma{}, nil)
		Kelp{}.NeighbourUpdateTick(pos, pos.Side(cube.FaceDown), tx)
		if _, ok := tx.Block(pos).(Kelp); ok {
			t.Fatal("kelp survived magma replacing its support")
		}
	})
}
