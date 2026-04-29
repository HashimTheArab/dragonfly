package block

import (
	"fmt"
	"os"
	"testing"
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

//go:linkname finaliseBlockRegistry github.com/df-mc/dragonfly/server/world.finaliseBlockRegistry
func finaliseBlockRegistry()

func TestMain(m *testing.M) {
	finaliseBlockRegistry()
	os.Exit(m.Run())
}

func testRedstoneTx(t *testing.T, f func(tx *world.Tx) error) {
	t.Helper()

	w := world.New()
	var txErr error
	<-w.Exec(func(tx *world.Tx) {
		txErr = f(tx)
	})
	if err := w.Close(); err != nil {
		t.Fatalf("close world: %v", err)
	}
	if txErr != nil {
		t.Fatal(txErr)
	}
}

func TestWoodDoorRedstonePowerChangesRespectManualToggle(t *testing.T) {
	testRedstoneTx(t, func(tx *world.Tx) error {
		bottomPos := cube.Pos{0, 1, 0}
		topPos := bottomPos.Side(cube.FaceUp)
		door := WoodDoor{Wood: OakWood(), Facing: cube.North}
		bottomPowerPos := bottomPos.Side(cube.FaceEast)
		topPowerPos := topPos.Side(cube.FaceEast)

		tx.SetBlock(bottomPos, door, nil)
		tx.SetBlock(topPos, WoodDoor{Wood: OakWood(), Facing: cube.North, Top: true}, nil)
		tx.SetBlock(bottomPowerPos, RedstoneBlock{}, nil)

		door.RedstoneUpdate(bottomPos, tx)
		poweredOpen := tx.Block(bottomPos).(WoodDoor)
		if !poweredOpen.Open {
			return fmt.Errorf("wooden door did not open on first redstone power")
		}

		poweredOpen.Activate(bottomPos, cube.FaceEast, tx, nil, nil)
		manuallyClosed := tx.Block(bottomPos).(WoodDoor)
		if manuallyClosed.Open {
			return fmt.Errorf("wooden door manual close while powered left it open")
		}

		manuallyClosed.RedstoneUpdate(bottomPos, tx)
		if stillClosed := tx.Block(bottomPos).(WoodDoor); stillClosed.Open {
			return fmt.Errorf("wooden door reopened without a fresh redstone power change")
		}

		tx.SetBlock(topPowerPos, RedstoneBlock{}, nil)
		tx.Block(bottomPos).(WoodDoor).RedstoneUpdate(bottomPos, tx)
		if reopened := tx.Block(bottomPos).(WoodDoor); !reopened.Open {
			return fmt.Errorf("wooden door did not reopen when a new power source was added")
		}

		tx.SetBlock(bottomPowerPos, Air{}, nil)
		tx.SetBlock(topPowerPos, Air{}, nil)
		tx.Block(bottomPos).(WoodDoor).RedstoneUpdate(bottomPos, tx)
		if closed := tx.Block(bottomPos).(WoodDoor); closed.Open {
			return fmt.Errorf("wooden door did not close when redstone power was removed")
		}
		return nil
	})
}

func TestWoodDoorOpenThenPoweredClosesWhenPowerRemoved(t *testing.T) {
	testRedstoneTx(t, func(tx *world.Tx) error {
		bottomPos := cube.Pos{0, 1, 0}
		topPos := bottomPos.Side(cube.FaceUp)
		door := WoodDoor{Wood: OakWood(), Facing: cube.North, Open: true}
		powerPos := bottomPos.Side(cube.FaceEast)

		tx.SetBlock(bottomPos, door, nil)
		tx.SetBlock(topPos, WoodDoor{Wood: OakWood(), Facing: cube.North, Top: true, Open: true}, nil)
		tx.SetBlock(powerPos, RedstoneBlock{}, nil)

		door.RedstoneUpdate(bottomPos, tx)
		if powered := tx.Block(bottomPos).(WoodDoor); !powered.Open {
			return fmt.Errorf("open wooden door closed when power was added")
		}

		tx.SetBlock(powerPos, Air{}, nil)
		tx.Block(bottomPos).(WoodDoor).RedstoneUpdate(bottomPos, tx)
		if closed := tx.Block(bottomPos).(WoodDoor); closed.Open {
			return fmt.Errorf("open wooden door did not close when added power was removed")
		}
		return nil
	})
}

func TestCopperDoorAcceptsHorizontalRedstonePowerOnEitherHalf(t *testing.T) {
	testRedstoneTx(t, func(tx *world.Tx) error {
		bottomPos := cube.Pos{0, 1, 0}
		topPos := bottomPos.Side(cube.FaceUp)
		door := CopperDoor{Facing: cube.North}

		tx.SetBlock(bottomPos, door, nil)
		tx.SetBlock(topPos, CopperDoor{Facing: cube.North, Top: true}, nil)
		tx.SetBlock(topPos.Side(cube.FaceEast), RedstoneBlock{}, nil)

		door.RedstoneUpdate(bottomPos, tx)
		if bottom := tx.Block(bottomPos).(CopperDoor); !bottom.Open {
			return fmt.Errorf("copper door did not open from horizontal redstone power on the top half")
		}
		if top := tx.Block(topPos).(CopperDoor); !top.Open {
			return fmt.Errorf("copper door top half did not mirror open state")
		}
		return nil
	})
}

func TestWoodTrapdoorRedstonePowerChangesRespectManualToggle(t *testing.T) {
	testRedstoneTx(t, func(tx *world.Tx) error {
		pos := cube.Pos{0, 1, 0}
		trapdoor := WoodTrapdoor{Wood: OakWood(), Facing: cube.North}
		powerPos := pos.Side(cube.FaceUp)
		secondPowerPos := pos.Side(cube.FaceEast)

		tx.SetBlock(pos, trapdoor, nil)
		tx.SetBlock(powerPos, RedstoneBlock{}, nil)

		trapdoor.RedstoneUpdate(pos, tx)
		poweredOpen := tx.Block(pos).(WoodTrapdoor)
		if !poweredOpen.Open {
			return fmt.Errorf("wood trapdoor did not open on first redstone power")
		}

		poweredOpen.Activate(pos, cube.FaceUp, tx, nil, nil)
		manuallyClosed := tx.Block(pos).(WoodTrapdoor)
		if manuallyClosed.Open {
			return fmt.Errorf("wood trapdoor manual close while powered left it open")
		}

		manuallyClosed.RedstoneUpdate(pos, tx)
		if stillClosed := tx.Block(pos).(WoodTrapdoor); stillClosed.Open {
			return fmt.Errorf("wood trapdoor reopened without a fresh redstone power change")
		}

		tx.SetBlock(secondPowerPos, RedstoneBlock{}, nil)
		tx.Block(pos).(WoodTrapdoor).RedstoneUpdate(pos, tx)
		if reopened := tx.Block(pos).(WoodTrapdoor); !reopened.Open {
			return fmt.Errorf("wood trapdoor did not reopen when a new power source was added")
		}

		tx.SetBlock(powerPos, Air{}, nil)
		tx.SetBlock(secondPowerPos, Air{}, nil)
		tx.Block(pos).(WoodTrapdoor).RedstoneUpdate(pos, tx)
		if closed := tx.Block(pos).(WoodTrapdoor); closed.Open {
			return fmt.Errorf("wood trapdoor did not close when redstone power was removed")
		}
		return nil
	})
}

func TestCopperTrapdoorRedstonePowerChangesRespectManualToggle(t *testing.T) {
	testRedstoneTx(t, func(tx *world.Tx) error {
		pos := cube.Pos{0, 1, 0}
		trapdoor := CopperTrapdoor{Facing: cube.North}
		powerPos := pos.Side(cube.FaceUp)
		secondPowerPos := pos.Side(cube.FaceEast)

		tx.SetBlock(pos, trapdoor, nil)
		tx.SetBlock(powerPos, RedstoneBlock{}, nil)

		trapdoor.RedstoneUpdate(pos, tx)
		poweredOpen := tx.Block(pos).(CopperTrapdoor)
		if !poweredOpen.Open {
			return fmt.Errorf("copper trapdoor did not open on first redstone power")
		}

		poweredOpen.Activate(pos, cube.FaceUp, tx, nil, nil)
		manuallyClosed := tx.Block(pos).(CopperTrapdoor)
		if manuallyClosed.Open {
			return fmt.Errorf("copper trapdoor manual close while powered left it open")
		}

		manuallyClosed.RedstoneUpdate(pos, tx)
		if stillClosed := tx.Block(pos).(CopperTrapdoor); stillClosed.Open {
			return fmt.Errorf("copper trapdoor reopened without a fresh redstone power change")
		}

		tx.SetBlock(secondPowerPos, RedstoneBlock{}, nil)
		tx.Block(pos).(CopperTrapdoor).RedstoneUpdate(pos, tx)
		if reopened := tx.Block(pos).(CopperTrapdoor); !reopened.Open {
			return fmt.Errorf("copper trapdoor did not reopen when a new power source was added")
		}
		return nil
	})
}
