package world

import (
	"slices"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

// populationTickObserver runs a test callback on each world-owner tick.
type populationTickObserver struct {
	NopHandler
	tick func(*Tx, int64)
}

// HandleTick forwards the owner transaction to the test callback.
func (h populationTickObserver) HandleTick(tx *Tx, tick int64) { h.tick(tx, tick) }

func TestPopulationTickHookUsesOwnerWithSharedSettings(t *testing.T) {
	provider := NopProvider{Set: defaultSettings()}
	first := Config{Synchronous: true, Provider: provider}.New()
	defer first.Close()
	second := Config{Synchronous: true, Provider: provider, Dim: Nether}.New()
	defer second.Close()
	var calls []int64
	second.Handle(populationTickObserver{tick: func(tx *Tx, tick int64) {
		calls = append(calls, tick)
		if tx.World() != second || tx.CurrentTick() != tick {
			t.Error("tick callback received a different world or tick")
		}
		// Reads may use world settings and admit terrain on this transaction.
		tx.World().Time()
		tx.Block(cube.Pos{})
	}})
	first.AdvanceTick()
	second.AdvanceTick()
	first.AdvanceTick()
	second.AdvanceTick()
	if len(calls) != 2 || calls[1] != calls[0]+1 {
		t.Fatalf("tick callbacks = %v, want two consecutive ticks", calls)
	}
}

func TestPopulationTickHookPreservesScheduledDelay(t *testing.T) {
	var ticks int
	scheduledTickTestBlockTicks = &ticks
	defer func() { scheduledTickTestBlockTicks = nil }()
	b := scheduledTickTestBlock{}
	w := Config{Synchronous: true, Blocks: scheduledTickTestRegistry()}.New()
	defer w.Close()
	w.Do(func(tx *Tx) {
		tx.SetBlock(cube.Pos{0, 10, 0}, b, &SetOpts{DisableBlockUpdates: true, DisableRedstoneUpdates: true})
	})
	scheduled := false
	w.Handle(populationTickObserver{tick: func(tx *Tx, _ int64) {
		if !scheduled {
			tx.ScheduleBlockUpdate(cube.Pos{0, 10, 0}, b, time.Second/20)
			scheduled = true
		}
	}})
	w.AdvanceTick()
	if ticks != 0 {
		t.Fatal("one-tick delay scheduled by the hook ran immediately")
	}
	w.AdvanceTick()
	if ticks != 1 {
		t.Fatalf("scheduled block ticked %d times, want once on the next tick", ticks)
	}
}

func TestPopulationTickingChunksUsesLoadedSnapshot(t *testing.T) {
	for _, synchronous := range []bool{false, true} {
		t.Run(map[bool]string{false: "loader", true: "synchronous"}[synchronous], func(t *testing.T) {
			w := Config{Synchronous: synchronous}.New()
			defer w.Close()
			w.SetPaused(true)
			w.SetTickRange(2)
			if !synchronous {
				loader := NewLoader(2, w, NopViewer{})
				defer w.Do(func(tx *Tx) { loader.Close(tx) })
			}
			w.Do(func(tx *Tx) {
				for _, pos := range []ChunkPos{{3, 0}, {0, 1}, {-1, 0}, {0, 0}} {
					tx.Block(cube.Pos{int(pos[0]) * 16, 0, int(pos[1]) * 16})
				}
				want := []ChunkPos{{-1, 0}, {0, 0}, {0, 1}}
				if synchronous {
					want = append(want, ChunkPos{3, 0})
				}
				if got := tx.TickingChunks(); !slices.Equal(got, want) {
					t.Errorf("ticking columns = %v, want %v", got, want)
				}
				if _, ok := tx.ChunkLastTick(ChunkPos{20, 20}); ok {
					t.Error("absent column reported a simulation tick")
				}
				if len(w.chunks) != 4 {
					t.Errorf("population queries changed loaded column count to %d", len(w.chunks))
				}
			})
		})
	}
}

func TestPopulationChunkLastTickTracksActiveColumns(t *testing.T) {
	w := Config{}.New()
	defer w.Close()
	w.SetPaused(true)
	w.SetTickRange(2)
	loader := NewLoader(2, w, NopViewer{})
	defer w.Do(func(tx *Tx) { loader.Close(tx) })
	w.Do(func(tx *Tx) {
		tx.Block(cube.Pos{})
		tx.Block(cube.Pos{160, 0, 0})
	})
	var firstTick int64
	w.Handle(populationTickObserver{tick: func(tx *Tx, tick int64) {
		if firstTick == 0 {
			firstTick = tick
		}
		near, _ := tx.ChunkLastTick(ChunkPos{})
		far, farTicked := tx.ChunkLastTick(ChunkPos{10, 0})
		if near != firstTick || (tick == firstTick && farTicked) || (tick > firstTick && (!farTicked || far != tick)) {
			t.Errorf("tick %d: origin=%d, moved-to=%d/%v", tick, near, far, farTicked)
		}
		if tick == firstTick+1 {
			tx.Block(cube.Pos{176, 0, 0})
			if _, ok := tx.ChunkLastTick(ChunkPos{11, 0}); ok {
				t.Error("column admitted during the callback was already stamped")
			}
		}
		if tick == firstTick+2 {
			if last, ok := tx.ChunkLastTick(ChunkPos{11, 0}); !ok || last != tick {
				t.Errorf("new resident column stamp = %d/%v, want %d/true", last, ok, tick)
			}
		}
	}})
	w.AdvanceTick()
	w.Do(func(tx *Tx) { loader.Move(tx, mgl64.Vec3{160, 0, 0}) })
	w.AdvanceTick()
	w.AdvanceTick()
	w.Handle(NopHandler{})
	w.AdvanceTick()
	w.Do(func(tx *Tx) {
		if last, _ := tx.ChunkLastTick(ChunkPos{10, 0}); last != firstTick+2 {
			t.Errorf("column stamped without a tick handler: %d, want %d", last, firstTick+2)
		}
	})
}
