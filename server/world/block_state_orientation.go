package world

import "github.com/df-mc/dragonfly/server/block/cube"

// PillarAxis returns the axis encoded by a pillar state, if present and valid.
func (s BlockState) PillarAxis() (cube.Axis, bool) {
	switch s.Properties["pillar_axis"] {
	case "x":
		return cube.X, true
	case "y":
		return cube.Y, true
	case "z":
		return cube.Z, true
	}
	return 0, false
}

// Facing returns the face encoded by a block state's directional property.
// It accepts canonical string properties and the legacy numeric encodings.
// A present string property takes precedence, including when its value is invalid.
func (s BlockState) Facing() (cube.Face, bool) {
	if name, ok := s.Properties["minecraft:cardinal_direction"].(string); ok {
		return stateFaceByName(name)
	}
	if name, ok := s.Properties["minecraft:facing_direction"].(string); ok {
		return stateFaceByName(name)
	}
	if index, ok := s.Properties["facing_direction"].(int32); ok && index >= int32(cube.FaceDown) && index <= int32(cube.FaceEast) {
		return cube.Face(FacingDirection(s.Name, cube.Face(index))), true
	}
	if index, ok := s.Properties["direction"].(int32); ok && index >= 0 && index < 4 {
		return [...]cube.Face{cube.FaceSouth, cube.FaceWest, cube.FaceNorth, cube.FaceEast}[index], true
	}
	return 0, false
}

// mirroredFacingBlocks number the horizontal values of facing_direction the
// other way round from every other block: 2 is south and 4 is east.
var mirroredFacingBlocks = map[string]struct{}{
	"minecraft:end_rod":                     {},
	"minecraft:piston":                      {},
	"minecraft:piston_arm_collision":        {},
	"minecraft:sticky_piston":               {},
	"minecraft:sticky_piston_arm_collision": {},
}

// FacingDirection returns the facing_direction value the named block uses for
// face. The mapping is its own inverse, so it also turns a value back into a face.
func FacingDirection(name string, face cube.Face) int32 {
	if _, mirrored := mirroredFacingBlocks[name]; mirrored && face.Axis() != cube.Y {
		face = face.Opposite()
	}
	return int32(face)
}

// stateFaceByName resolves a string-valued block-state direction.
func stateFaceByName(name string) (cube.Face, bool) {
	for _, face := range cube.Faces() {
		if face.String() == name {
			return face, true
		}
	}
	return 0, false
}
