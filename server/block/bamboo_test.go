package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func TestBambooUsesBambooModelForInteractionTargeting(t *testing.T) {
	if _, ok := (Bamboo{}).Model().(model.Bamboo); !ok {
		t.Fatalf("Bamboo.Model() = %T, want model.Bamboo", (Bamboo{}).Model())
	}
}

func TestBambooSaplingUsesBambooModelForInteractionTargeting(t *testing.T) {
	if _, ok := (BambooSapling{}).Model().(model.Bamboo); !ok {
		t.Fatalf("BambooSapling.Model() = %T, want model.Bamboo", (BambooSapling{}).Model())
	}
}

func TestUsingBambooOnExistingBambooDoesNotReplaceClickedStalk(t *testing.T) {
	w := world.Config{DisableLighting: true}.New()
	defer w.Close()

	var testErr error
	<-w.Exec(func(tx *world.Tx) {
		pos := cube.Pos{0, 1, 0}
		tx.SetBlock(pos.Side(cube.FaceDown), Dirt{}, nil)
		tx.SetBlock(pos, Bamboo{Age: false, LeafSize: bambooNoLeaves, Thick: false}, nil)

		ctx := &item.UseContext{}
		_ = (Bamboo{}).UseOnBlock(pos, cube.FaceUp, mgl64.Vec3{}, tx, nil, ctx)
		if _, ok := tx.Block(pos).(Bamboo); !ok {
			testErr = errString("clicked bamboo was replaced by " + blockType(tx.Block(pos)))
			return
		}
		if _, ok := tx.Block(pos.Side(cube.FaceUp)).(Bamboo); !ok {
			testErr = errString("using bamboo on bamboo did not extend the stalk")
			return
		}
	})
	if testErr != nil {
		t.Fatal(testErr)
	}
}

func TestUsingBambooOnBambooSaplingConvertsSaplingToStalk(t *testing.T) {
	w := world.Config{DisableLighting: true}.New()
	defer w.Close()

	var testErr error
	<-w.Exec(func(tx *world.Tx) {
		pos := cube.Pos{0, 1, 0}
		tx.SetBlock(pos.Side(cube.FaceDown), Dirt{}, nil)
		tx.SetBlock(pos, BambooSapling{Age: false}, nil)

		ctx := &item.UseContext{}
		_ = (Bamboo{}).UseOnBlock(pos, cube.FaceUp, mgl64.Vec3{}, tx, nil, ctx)
		if _, ok := tx.Block(pos).(Bamboo); !ok {
			testErr = errString("clicked bamboo sapling was not converted to bamboo stalk: " + blockType(tx.Block(pos)))
			return
		}
		if _, ok := tx.Block(pos.Side(cube.FaceUp)).(Bamboo); !ok {
			testErr = errString("using bamboo on bamboo sapling did not place a bamboo stalk above")
			return
		}
	})
	if testErr != nil {
		t.Fatal(testErr)
	}
}

func TestBoneMealItemOnBambooSaplingConvertsSaplingToStalk(t *testing.T) {
	w := world.Config{DisableLighting: true}.New()
	defer w.Close()

	var testErr error
	<-w.Exec(func(tx *world.Tx) {
		pos := cube.Pos{0, 1, 0}
		tx.SetBlock(pos.Side(cube.FaceDown), Dirt{}, nil)
		tx.SetBlock(pos, BambooSapling{Age: false}, nil)

		ctx := &item.UseContext{}
		if ok := (item.BoneMeal{}).UseOnBlock(pos, cube.FaceUp, mgl64.Vec3{}, tx, nil, ctx); !ok {
			testErr = errString("bone meal item did not affect bamboo sapling")
			return
		}
		if ctx.CountSub != 1 {
			testErr = errString("bone meal item was not consumed")
			return
		}
		if _, ok := tx.Block(pos).(Bamboo); !ok {
			testErr = errString("bone meal did not convert bamboo sapling to stalk: " + blockType(tx.Block(pos)))
			return
		}
		if _, ok := tx.Block(pos.Side(cube.FaceUp)).(Bamboo); !ok {
			testErr = errString("bone meal did not grow bamboo above sapling")
			return
		}
	})
	if testErr != nil {
		t.Fatal(testErr)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func blockType(b world.Block) string {
	if b == nil {
		return "<nil>"
	}
	name, _ := b.EncodeBlock()
	return name
}
