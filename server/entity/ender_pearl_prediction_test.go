package entity

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type pearlPredictionBlocks map[cube.Pos]world.Block

func (blocks pearlPredictionBlocks) Block(pos cube.Pos) world.Block {
	if b, ok := blocks[pos]; ok {
		return b
	}
	return block.Air{}
}

func TestPredictEnderPearlLanding_BlockAndUnknownTerrain(t *testing.T) {
	start, velocity := mgl64.Vec3{0.5, 0.5, 0.5}, mgl64.Vec3{0, 0, 1}
	blocks := pearlPredictionBlocks{{0, 0, 3}: block.Stone{}}
	hit, ok := PredictEnderPearlLanding(blocks, func(cube.Pos) bool { return true }, start, velocity, 0, 0, 8)
	if !ok || !hit.Hit || hit.Position != (mgl64.Vec3{0.5, 0.5, 3}) || hit.Block != (cube.Pos{0, 0, 3}) {
		t.Fatalf("landing = %v, %t", hit, ok)
	}
	partial, ok := PredictEnderPearlLanding(blocks, func(pos cube.Pos) bool { return pos.Z() < 2 }, start, velocity, 0, 0, 8)
	if !ok || partial.Hit || partial.Position != (mgl64.Vec3{0.5, 0.5, 1.5}) {
		t.Fatalf("unknown terrain = %v, %t", partial, ok)
	}
}

func TestPredictEnderPearlLanding_BDSMovementBeforeForces(t *testing.T) {
	for _, tt := range []struct {
		name          string
		velocity      mgl64.Vec3
		gravity, drag float64
		ticks         int
		want          mgl64.Vec3
	}{
		{"bds first tick", mgl64.Vec3{0, 0, 1.5}, 0.025, 0, 1, mgl64.Vec3{0.5, 10.5, 2}},
		{"bds third tick", mgl64.Vec3{0, 0, 1.5}, 0.025, 0, 3, mgl64.Vec3{0.5, 10.425000190734863, 5}},
		{"custom first tick", mgl64.Vec3{2, 1, 3}, 0.25, 0.5, 1, mgl64.Vec3{2.5, 11.5, 3.5}},
		{"custom third tick", mgl64.Vec3{2, 1, 3}, 0.25, 0.5, 3, mgl64.Vec3{4, 11.625, 5.75}},
		{"stationary apex", mgl64.Vec3{}, 0.25, 0.5, 2, mgl64.Vec3{0.5, 10.25, 0.5}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := PredictEnderPearlLanding(pearlPredictionBlocks{}, func(cube.Pos) bool { return true }, mgl64.Vec3{0.5, 10.5, 0.5}, tt.velocity, tt.gravity, tt.drag, tt.ticks)
			if !ok || got.Hit || got.Position.Sub(tt.want).Len() > 1e-10 {
				t.Fatalf("flight after %d ticks = %v, %t; want %v", tt.ticks, got, ok, tt.want)
			}
		})
	}
}

func TestPredictEnderPearlLanding_FirstTickBlockAtLaunchHeight(t *testing.T) {
	blocks := pearlPredictionBlocks{{0, 1, 1}: block.Stone{}}
	got, ok := PredictEnderPearlLanding(blocks, func(cube.Pos) bool { return true }, mgl64.Vec3{0.5, 1.01, 0.5}, mgl64.Vec3{0, 0, 1}, 0.025, 0, 1)
	want := mgl64.Vec3{0.5, 1.0099999904632568, 1}
	if !ok || !got.Hit || got.Block != (cube.Pos{0, 1, 1}) || got.Position.Sub(want).Len() > 1e-10 {
		t.Fatalf("first tick collision = %v, %t; want block {0, 1, 1} at %v", got, ok, want)
	}
}

type pearlPredictionBDSWorld struct {
	FloorY    int `json:"floor_y"`
	FloorMinX int `json:"floor_min_x"`
	FloorMaxX int `json:"floor_max_x"`
	FloorMinZ int `json:"floor_min_z"`
	FloorMaxZ int `json:"floor_max_z"`
	Wall      *struct {
		Z    int `json:"z"`
		MinY int `json:"min_y"`
		MaxY int `json:"max_y"`
		MinX int `json:"min_x"`
		MaxX int `json:"max_x"`
	} `json:"wall"`
}

func (w pearlPredictionBDSWorld) Block(pos cube.Pos) world.Block {
	if pos.Y() == w.FloorY && pos.X() >= w.FloorMinX && pos.X() <= w.FloorMaxX && pos.Z() >= w.FloorMinZ && pos.Z() <= w.FloorMaxZ {
		return block.Stone{}
	}
	if wall := w.Wall; wall != nil && pos.Z() == wall.Z && pos.Y() >= wall.MinY && pos.Y() <= wall.MaxY && pos.X() >= wall.MinX && pos.X() <= wall.MaxX {
		return block.Stone{}
	}
	return block.Air{}
}

// The fixture stores six direct official-BDS throws, with the launch velocity
// from AddActor and the landing from the player's teleport. It contains no
// account or connection identifiers.
func TestPredictEnderPearlLanding_ObservedBDSLaunchAndImpact(t *testing.T) {
	data, err := os.ReadFile("testdata/ender_pearl_bds_1_26_50.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Samples []struct {
			Rotation struct {
				Pitch, Yaw float64
			}
			Start            mgl64.Vec3
			CapturedVelocity mgl64.Vec3 `json:"captured_velocity"`
			World            pearlPredictionBDSWorld
			Contact          mgl64.Vec3 `json:"actual_impact"`
			Block            cube.Pos   `json:"actual_block"`
			Ticks            int        `json:"flight_ticks"`
		} `json:"samples"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Samples) != 6 {
		t.Fatalf("observed throws = %d, want 6", len(fixture.Samples))
	}
	for i, shot := range fixture.Samples {
		t.Run(fmt.Sprintf("shot%d_pitch%g_yaw%g", i+1, shot.Rotation.Pitch, shot.Rotation.Yaw), func(t *testing.T) {
			velocity := EnderPearlLaunchVelocity(cube.Rotation{shot.Rotation.Yaw, shot.Rotation.Pitch}, 1.5)
			if velocity != shot.CapturedVelocity {
				t.Fatalf("launch velocity = %v; observed %v", velocity, shot.CapturedVelocity)
			}
			available := func(cube.Pos) bool { return true }
			before, ok := PredictEnderPearlLanding(shot.World, available, shot.Start, velocity, 0.025, 0, shot.Ticks-1)
			if !ok || before.Hit {
				t.Fatalf("predicted impact before fixture tick %d: %+v, %t", shot.Ticks, before, ok)
			}
			got, ok := PredictEnderPearlLanding(shot.World, available, shot.Start, velocity, 0.025, 0, shot.Ticks)
			if !ok || !got.Hit || got.Block != shot.Block || got.Position != shot.Contact {
				t.Fatalf("impact = %+v, %t; observed block %v at %v with fixture flight %d ticks", got, ok, shot.Block, shot.Contact, shot.Ticks)
			}
		})
	}
}

func TestPredictEnderPearlLanding_GrazesBlockEdge(t *testing.T) {
	// Translate the observed upward diagonal throw to within one millimetre
	// of this block's edge. Continuous trigonometry misses the block entirely.
	start := mgl64.Vec3{1300.654, 65.62001037597656, 1300.5}
	rotation := cube.Rotation{30, -20}
	blocks := pearlPredictionBlocks{{1269, 63, 1355}: block.Stone{}}
	available := func(cube.Pos) bool { return true }
	got, ok := PredictEnderPearlLanding(blocks, available, start, EnderPearlLaunchVelocity(rotation, 1.5), 0.025, 0, 240)
	want := mgl64.Vec3{1269.0008544921875, 64, 1355.3297119140625}
	if !ok || !got.Hit || got.Block != (cube.Pos{1269, 63, 1355}) || got.Position != want {
		t.Fatalf("grazing impact = %+v, %t; want block {1269, 63, 1355} at %v", got, ok, want)
	}
	continuous, ok := PredictEnderPearlLanding(blocks, available, start, rotation.Vec3().Mul(1.5), 0.025, 0, 240)
	if !ok || continuous.Hit {
		t.Fatalf("continuous direction must miss this grazing block: %+v, %t", continuous, ok)
	}
}

func TestPredictEnderPearlLanding_RejectsInvalidSinglePrecision(t *testing.T) {
	for _, tt := range []struct {
		name          string
		start, motion mgl64.Vec3
		gravity, drag float64
	}{
		{name: "nan gravity", gravity: math.NaN()},
		{name: "infinite gravity", gravity: math.Inf(1)},
		{name: "nan drag", drag: math.NaN()},
		{name: "gravity overflows float32", gravity: math.MaxFloat64},
		{name: "start overflows float32", start: mgl64.Vec3{math.MaxFloat64}},
		{name: "motion overflows float32", motion: mgl64.Vec3{math.MaxFloat64}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			available := func(cube.Pos) bool {
				t.Fatal("invalid inputs reached terrain traversal")
				return false
			}
			if got, ok := PredictEnderPearlLanding(pearlPredictionBlocks{}, available, tt.start, tt.motion, tt.gravity, tt.drag, 2); ok {
				t.Fatalf("invalid inputs were accepted: %+v", got)
			}
		})
	}
	velocity := EnderPearlLaunchVelocity(cube.Rotation{math.NaN(), 0}, 1.5)
	if _, ok := PredictEnderPearlLanding(pearlPredictionBlocks{}, func(cube.Pos) bool { return true }, mgl64.Vec3{}, velocity, 0.025, 0, 2); ok {
		t.Fatal("invalid launch rotation produced a usable prediction")
	}
}
