package customblock

import (
	"fmt"
	"math"
	"slices"
)

// Trait is a vanilla block trait. It adds states to a custom block that the client sets by the
// trait's own rule when a player places the block. Only the traits of this package exist, as the
// client knows no others.
type Trait interface {
	// Encode returns the trait encoded as an entry of the block's traits list.
	Encode() map[string]any
	// States returns the states the trait adds, in the order the client adds them to the block.
	States() []TraitState
	// rank is the trait's place in vanilla's trait order.
	rank() int
	validate() error
}

// TraitState is a block state a trait adds, with its values in the order the client enumerates
// them: the order of its Direction and Facing enums, which vanilla's own block states share.
type TraitState struct {
	Name   string
	Values []any
}

// SortTraits returns the traits in the order vanilla lists them and adds their states to a block,
// whatever order they were declared in: placement_position before placement_direction. It returns
// an error if a trait is invalid or appears twice.
func SortTraits(traits []Trait) ([]Trait, error) {
	sorted := slices.Clone(traits)
	slices.SortStableFunc(sorted, func(a, b Trait) int { return a.rank() - b.rank() })
	for i, t := range sorted {
		if i > 0 && sorted[i-1].rank() == t.rank() {
			return nil, fmt.Errorf("trait %v appears twice", t.Encode()["name"])
		}
		if err := t.validate(); err != nil {
			return nil, fmt.Errorf("trait %v: %w", t.Encode()["name"], err)
		}
	}
	return sorted, nil
}

var (
	cardinalDirections = []any{"south", "west", "north", "east"}
	facingDirections   = []any{"down", "up", "north", "south", "west", "east"}
)

// PlacementDirection is the minecraft:placement_direction trait. Each enabled state is set from
// the direction the placing player faces. At least one state must be enabled.
type PlacementDirection struct {
	// CardinalDirection enables the minecraft:cardinal_direction state, one of "south", "west",
	// "north" and "east".
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
			"cardinal_direction":            boolByte(t.CardinalDirection),
			"facing_direction":              boolByte(t.FacingDirection),
			"corner_and_cardinal_direction": uint8(0),
			"sixteen_way_rotation":          uint8(0),
		},
		"y_rotation_offset":     float32(t.YRotationOffset),
		"blocks_to_corner_with": []any{},
	}
}

// States returns the enabled states, cardinal before facing, as the client adds them.
func (t PlacementDirection) States() []TraitState {
	var states []TraitState
	if t.CardinalDirection {
		states = append(states, TraitState{Name: "minecraft:cardinal_direction", Values: cardinalDirections})
	}
	if t.FacingDirection {
		states = append(states, TraitState{Name: "minecraft:facing_direction", Values: facingDirections})
	}
	return states
}

func (PlacementDirection) rank() int { return 1 }

func (t PlacementDirection) validate() error {
	if !t.CardinalDirection && !t.FacingDirection {
		return fmt.Errorf("no state enabled")
	}
	if t.YRotationOffset < 0 || t.YRotationOffset > 360 || math.Mod(t.YRotationOffset, 90) != 0 {
		return fmt.Errorf("y rotation offset %v is not one of 0, 90, 180, 270 and 360", t.YRotationOffset)
	}
	return nil
}

// PlacementPosition is the minecraft:placement_position trait. Each enabled state is set from
// where the placing player clicked. At least one state must be enabled.
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

// States returns the enabled states, block face before vertical half, as the client adds them.
func (t PlacementPosition) States() []TraitState {
	var states []TraitState
	if t.BlockFace {
		states = append(states, TraitState{Name: "minecraft:block_face", Values: facingDirections})
	}
	if t.VerticalHalf {
		states = append(states, TraitState{Name: "minecraft:vertical_half", Values: []any{"bottom", "top"}})
	}
	return states
}

func (PlacementPosition) rank() int { return 0 }

func (t PlacementPosition) validate() error {
	if !t.BlockFace && !t.VerticalHalf {
		return fmt.Errorf("no state enabled")
	}
	return nil
}

// boolByte returns b as the byte a client reads an enabled state flag as.
func boolByte(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
