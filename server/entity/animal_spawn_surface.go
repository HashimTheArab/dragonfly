package entity

import (
	"math"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// animalSurface finds the first upward-supporting material below transparent
// foliage and vegetation. Liquids terminate the surface search and are rejected
// by the species' supporting-block filter.
func animalSurface(tx *world.Tx, x, z int) (cube.Pos, bool) {
	source := &animalBlockSource{tx: tx}
	for y := tx.HighestBlock(x, z); y >= tx.Range().Min(); y-- {
		pos := cube.Pos{x, y, z}
		b := tx.Block(pos)
		if _, liquid := b.(world.Liquid); liquid {
			return pos, true
		}
		solid := b.Model().FaceSolid(pos, cube.FaceUp, source)
		if source.missing {
			return cube.Pos{}, false
		}
		if solid {
			return pos, true
		}
	}
	return cube.Pos{}, false
}

// animalVolumeClear tests the candidate's block collision volume without
// reserving space between herd members. Entity overlap is allowed at admission.
func animalVolumeClear(tx *world.Tx, feet cube.Pos, species animalType) bool {
	at := mgl64.Vec3{float64(feet[0]) + spawnHorizontalOffset, float64(feet[1]), float64(feet[2]) + spawnHorizontalOffset}
	box := species.BBox(nil).Translate(at).Grow(-.00001)
	source := &animalBlockSource{tx: tx}
	for pos := range cube.Range3D(cube.PosFromVec3(box.Min()), cube.PosFromVec3(box.Max())) {
		b := source.Block(pos)
		if source.missing {
			return false
		}
		if _, liquid := tx.Liquid(pos); liquid {
			return false
		}
		bounds := b.Model().BBox(pos, source)
		if source.missing {
			return false
		}
		for _, bounds := range bounds {
			if bounds.Translate(pos.Vec3()).IntersectsWith(box) {
				return false
			}
		}
	}
	return true
}

// animalBlockSource bounds collision-model reads to resident terrain. A missing
// neighbour rejects the spawn attempt instead of generating another column.
type animalBlockSource struct {
	tx      *world.Tx
	missing bool
}

// Block returns a loaded block and records any unavailable collision dependency.
func (s *animalBlockSource) Block(pos cube.Pos) world.Block {
	b, ok := s.tx.BlockLoaded(pos)
	if !ok {
		s.missing = true
		return block.Air{}
	}
	return b
}

// animalBrightness combines block light with daylight and the host's weather
// state. Authored rules disable the separate thunderstorm brightness override.
func animalBrightness(tx *world.Tx, pos cube.Pos) int {
	sky, block := tx.SkyLight(pos), tx.BlockLight(pos)
	day := float32(tx.World().Time()%24000)/24000 - .25
	if day < 0 {
		day++
	}
	if day > 1 {
		day--
	}
	angle := day + ((1-(spawnCos(day*float32(math.Pi))+1)/2)-day)/3
	light := float32(1) - max(float32(0), min(float32(1), 1-(spawnCos(angle*float32(math.Pi)*2)*2+.5)))
	if tx.Raining() {
		light *= 1 - 5.0/16
	}
	if tx.Thundering() {
		light *= 1 - 5.0/16
	}
	darkening := int((1 - light) * 11)
	return max(int(block), max(0, int(sky)-darkening))
}

// spawnCos evaluates the quantised trigonometric phase used for daylight gates.
// The table value is generated mathematically rather than copied from assets.
func spawnCos(angle float32) float32 {
	index := int(angle*10430.3779296875+16384) & 65535
	return float32(math.Sin(float64(index) * 2 * math.Pi / 65536))
}
