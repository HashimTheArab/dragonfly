package world

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
)

// optionalSource emits full power and acts on input, but only takes part in redstone when on.
type optionalSource struct{ on bool }

func (optionalSource) EncodeBlock() (string, map[string]any) { return "test:optional_source", nil }
func (optionalSource) Hash() (uint64, uint64)                { return 0, 0 }
func (optionalSource) Model() BlockModel                     { return nil }
func (s optionalSource) RedstoneComponent() bool             { return s.on }
func (optionalSource) RedstonePower(cube.Pos, *Tx, cube.Face) int {
	return 15
}
func (optionalSource) RedstonePowerAction(cube.Pos, *Tx, int, int) {}

// A block that opts out of redstone is classified as no component, whatever interfaces its type
// implements; one that opts in, or does not implement RedstoneOptional, keeps them.
func TestRedstoneOptionalBlocksOptOut(t *testing.T) {
	if isRedstoneRelevant(optionalSource{on: false}) || RedstoneComponent(optionalSource{on: false}) {
		t.Fatal("an opted-out block is still a redstone component")
	}
	source, _, action, _ := classifyRedstoneBlock(optionalSource{on: true})
	if !source || !action || !RedstoneComponent(optionalSource{on: true}) {
		t.Fatal("an opted-in block lost its redstone interfaces")
	}
	if !RedstoneComponent(nil) {
		t.Fatal("a block without RedstoneOptional is not a redstone component")
	}
}

// optionalRelayer carries power, but only takes part in redstone when on.
type optionalRelayer struct{ on bool }

func (r optionalRelayer) EncodeBlock() (string, map[string]any) {
	return "test:optional_relayer", map[string]any{"on": r.on}
}
func (r optionalRelayer) Hash() (uint64, uint64) {
	return 1 << 50, map[bool]uint64{false: 0, true: 1}[r.on]
}
func (optionalRelayer) Model() BlockModel                    { return redstoneCancellationModel{} }
func (r optionalRelayer) RedstoneComponent() bool            { return r.on }
func (optionalRelayer) RedstoneSignalLoss(cube.Pos, *Tx) int { return 1 }

// optionalStrongSource strongly powers its neighbours, but only takes part in redstone when on.
type optionalStrongSource struct{ on bool }

func (s optionalStrongSource) EncodeBlock() (string, map[string]any) {
	return "test:optional_strong_source", map[string]any{"on": s.on}
}
func (s optionalStrongSource) Hash() (uint64, uint64) {
	return 1 << 51, map[bool]uint64{false: 0, true: 1}[s.on]
}
func (optionalStrongSource) Model() BlockModel         { return redstoneCancellationModel{} }
func (s optionalStrongSource) RedstoneComponent() bool { return s.on }
func (optionalStrongSource) RedstonePower(cube.Pos, *Tx, cube.Face) int {
	return 15
}
func (optionalStrongSource) RedstoneStrongPower(cube.Pos, *Tx, cube.Face) int { return 15 }

func optionalRedstoneTestRegistry() BlockRegistry {
	registry := redstoneCancellationTestRegistry()
	for _, on := range []bool{false, true} {
		registry.RegisterBlockState(BlockState{Name: "test:optional_relayer", Properties: map[string]any{"on": on}})
		registry.RegisterBlock(optionalRelayer{on: on})
		registry.RegisterBlockState(BlockState{Name: "test:optional_strong_source", Properties: map[string]any{"on": on}})
		registry.RegisterBlock(optionalStrongSource{on: on})
	}
	return registry
}

// An opted-out block neither gives strong power nor carries power on as a relayer.
func TestRedstoneOptionalBlocksGiveNoStrongPowerOrRelay(t *testing.T) {
	for _, on := range []bool{false, true} {
		w := Config{Synchronous: true, Blocks: optionalRedstoneTestRegistry()}.New()
		want := 0
		if on {
			want = 15
		}
		runWorld(w, func(tx *Tx) {
			tx.SetBlock(cube.Pos{1, 0, 0}, optionalStrongSource{on: on}, nil)
			e := &redstoneEngine{}
			if got := e.strongPowerFrom(cube.Pos{0, 0, 0}, tx, cube.FaceEast); got != want {
				t.Errorf("on=%v: strong power from source = %d, want %d", on, got, want)
			}
			if _, ok := redstoneRelayer(optionalRelayer{on: on}); ok != on {
				t.Errorf("on=%v: relayer = %v, want %v", on, ok, on)
			}
		})
		_ = w.Close()
	}
}
