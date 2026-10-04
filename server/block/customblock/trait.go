package customblock

// Trait is a vanilla block trait. It adds states to a custom block that the client sets by the
// trait's own rule when a player places the block.
type Trait interface {
	// Encode returns the trait encoded as an entry of the block's traits list.
	Encode() map[string]any
}

// PlacementDirection is the minecraft:placement_direction trait. Each enabled state is set from
// the direction the placing player faces.
type PlacementDirection struct {
	// CardinalDirection enables the minecraft:cardinal_direction state, one of "north", "east",
	// "south" and "west".
	CardinalDirection bool
	// FacingDirection enables the minecraft:facing_direction state, one of "down", "up",
	// "north", "south", "west" and "east".
	FacingDirection bool
	// YRotationOffset rotates the direction the states are set from around the Y axis, in
	// degrees. It must be one of 0, 90, 180, 270 and 360.
	YRotationOffset float64
}

// Encode returns the trait encoded as an entry of the block's traits list.
func (t PlacementDirection) Encode() map[string]any {
	return map[string]any{
		"name": "minecraft:placement_direction",
		"enabled_states": map[string]any{
			"cardinal_direction": boolByte(t.CardinalDirection),
			"facing_direction":   boolByte(t.FacingDirection),
		},
		"y_rotation_offset": float32(t.YRotationOffset),
	}
}

// PlacementPosition is the minecraft:placement_position trait. Each enabled state is set from
// where the placing player clicked.
type PlacementPosition struct {
	// BlockFace enables the minecraft:block_face state: the face of the clicked block the new
	// block was placed against, one of "down", "up", "north", "south", "west" and "east".
	BlockFace bool
	// VerticalHalf enables the minecraft:vertical_half state, "bottom" or "top": the half of
	// the clicked face the player clicked.
	VerticalHalf bool
}

// Encode returns the trait encoded as an entry of the block's traits list.
func (t PlacementPosition) Encode() map[string]any {
	return map[string]any{
		"name": "minecraft:placement_position",
		"enabled_states": map[string]any{
			"block_face":    boolByte(t.BlockFace),
			"vertical_half": boolByte(t.VerticalHalf),
		},
	}
}

// boolByte returns b as the byte a client reads an enabled state flag as.
func boolByte(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
