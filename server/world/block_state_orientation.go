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
		return cube.Face(index), true
	}
	if index, ok := s.Properties["direction"].(int32); ok && index >= 0 && index < 4 {
		return [...]cube.Face{cube.FaceSouth, cube.FaceWest, cube.FaceNorth, cube.FaceEast}[index], true
	}
	return 0, false
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
