package world

import (
	"reflect"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
)

func TestRunBlockTransaction_Reliability(t *testing.T) {
	t.Parallel()

	view := &blockTransactionTestView{blocks: map[cube.Pos]Block{}}
	if !RunBlockTransaction(view, func(tx *Tx) {
		tx.SetBlock(cube.Pos{}, nil, nil)
		if _, ok := tx.BlockLoaded(cube.Pos{}); !ok {
			t.Fatal("detached write was not readable")
		}
	}) {
		t.Fatal("supported detached transaction was marked incomplete")
	}
	if RunBlockTransaction(view, func(tx *Tx) { _ = tx.Light(cube.Pos{}) }) {
		t.Fatal("unsupported detached light query was reported complete")
	}
	if RunBlockTransaction(nil, func(*Tx) {}) || RunBlockTransaction(view, nil) {
		t.Fatal("nil detached transaction input was reported complete")
	}
}

func TestBlockTransactionView_TxMethodCoverage(t *testing.T) {
	t.Parallel()

	// Every exported Tx method must be classified when detached transactions
	// are changed. This prevents a new method from silently falling through to
	// the synthetic world's empty state.
	classified := map[string]string{
		"Event":          "supported",
		"Range":          "supported",
		"SetBlock":       "supported",
		"SetBlockEntity": "supported",
		"Block":          "supported",
		"BlockLoaded":    "supported",
		"Liquid":         "supported",
		"SetLiquid":      "supported",
		"Redstone":       "supported handle; operations invalidate",

		"ScheduleBlockUpdate": "discarded side effect",
		"AddParticle":         "discarded side effect",
		"PlayEntityAnimation": "discarded side effect",
		"PlaySound":           "discarded side effect",

		"Defer":                      "unsupported",
		"DeferErr":                   "unsupported",
		"BlocksWithin":               "unsupported",
		"BuildStructure":             "unsupported",
		"HighestLightBlocker":        "unsupported",
		"HighestBlock":               "unsupported",
		"Light":                      "unsupported",
		"SkyLight":                   "unsupported",
		"SetBiome":                   "unsupported",
		"Biome":                      "unsupported",
		"Temperature":                "unsupported",
		"RainingAt":                  "unsupported",
		"SnowingAt":                  "unsupported",
		"ThunderingAt":               "unsupported",
		"Raining":                    "unsupported",
		"Thundering":                 "unsupported",
		"AddEntity":                  "unsupported",
		"AddEntityAt":                "unsupported",
		"RemoveEntity":               "unsupported",
		"EntitiesWithin":             "unsupported",
		"Entities":                   "unsupported",
		"Players":                    "unsupported",
		"Viewers":                    "unsupported",
		"Sleepers":                   "unsupported",
		"BroadcastSleepingIndicator": "unsupported",
		"BroadcastSleepingReminder":  "unsupported",
		"World":                      "supported synthetic dimension and range",
		"CurrentTick":                "unsupported",
		"RedstonePower":              "unsupported",
		"RedstoneDirectPower":        "unsupported",
		"RedstoneStrongPower":        "unsupported",
		"RedstoneConductivePower":    "unsupported",
		"RedstonePowerFrom":          "unsupported",
		"RedstoneDirectPowerFrom":    "unsupported",
		"RedstoneStrongPowerFrom":    "unsupported",
	}
	typ := reflect.TypeOf((*Tx)(nil))
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if _, ok := classified[method.Name]; !ok {
			t.Errorf("Tx.%s is not classified for detached transactions", method.Name)
		}
	}
	for name := range classified {
		if _, ok := typ.MethodByName(name); !ok {
			t.Errorf("stale detached Tx classification for removed method %s", name)
		}
	}
}

type blockTransactionTestView struct {
	blocks map[cube.Pos]Block
}

func (*blockTransactionTestView) Range() cube.Range    { return Overworld.Range() }
func (*blockTransactionTestView) Dimension() Dimension { return Overworld }

func (v *blockTransactionTestView) Block(pos cube.Pos) Block {
	b, _ := v.BlockLoaded(pos)
	return b
}

func (v *blockTransactionTestView) BlockLoaded(pos cube.Pos) (Block, bool) {
	b, ok := v.blocks[pos]
	return b, ok
}

func (*blockTransactionTestView) Liquid(cube.Pos) (Liquid, bool) { return nil, false }

func (v *blockTransactionTestView) SetBlock(pos cube.Pos, b Block) {
	v.blocks[pos] = b
}

func (*blockTransactionTestView) SetLiquid(cube.Pos, Liquid) {}
