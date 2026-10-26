package customblock

var (
	cornerShapes         = []any{"none", "inner_left", "inner_right", "outer_left", "outer_right"}
	verticalHalves       = []any{"bottom", "top"}
	connectionValues     = []any{uint8(0), uint8(1)}
	sixteenWayRotations  = indexValues(16)
	connectionDirections = []string{"north", "east", "south", "west"}
)

// NetworkTraitStates returns the states a trait entry of a StartGame block definition adds, in the
// order the client adds them, reading the entry as the client does. name is the trait's lowercased
// name. A trait the client does not know adds no states, and a flag the client reads as a byte is
// only set by a non-zero byte.
func NetworkTraitStates(name string, trait map[string]any) []TraitState {
	switch name {
	case "minecraft:placement_direction":
		enabled, ok := trait["enabled_states"].(map[string]any)
		if !ok {
			return nil
		}
		corner := networkFlag(enabled["corner_and_cardinal_direction"])
		var states []TraitState
		if corner || networkFlag(enabled["cardinal_direction"]) {
			states = append(states, TraitState{Name: "minecraft:cardinal_direction", Values: cardinalDirections})
		}
		if corner {
			states = append(states, TraitState{Name: "minecraft:corner", Values: cornerShapes})
		}
		if networkFlag(enabled["facing_direction"]) {
			states = append(states, TraitState{Name: "minecraft:facing_direction", Values: facingDirections})
		}
		if networkFlag(enabled["sixteen_way_rotation"]) {
			states = append(states, TraitState{Name: "minecraft:sixteen_way_rotation", Values: sixteenWayRotations})
		}
		return states
	case "minecraft:placement_position":
		enabled, ok := trait["enabled_states"].(map[string]any)
		if !ok {
			return nil
		}
		var states []TraitState
		if networkFlag(enabled["block_face"]) {
			states = append(states, TraitState{Name: "minecraft:block_face", Values: facingDirections})
		}
		if networkFlag(enabled["vertical_half"]) {
			states = append(states, TraitState{Name: "minecraft:vertical_half", Values: verticalHalves})
		}
		return states
	case "minecraft:connection":
		// enabled_states is an int, of which the client only tests the lowest bit.
		enabled, _ := trait["enabled_states"].(int32)
		if enabled&1 == 0 {
			return nil
		}
		states := make([]TraitState, 0, len(connectionDirections))
		for _, direction := range connectionDirections {
			states = append(states, TraitState{Name: "minecraft:connection_" + direction, Values: connectionValues})
		}
		return states
	case "minecraft:multi_block":
		enabled, ok := trait["enabled_states"].(map[string]any)
		if !ok || !networkFlag(enabled["multi_block"]) {
			return nil
		}
		parts, _ := trait["part_count"].(uint8)
		return []TraitState{{Name: "minecraft:multi_block_part", Values: indexValues(int(parts))}}
	}
	return nil
}

// networkFlag reports whether v is a set byte flag. Any other tag type reads as unset.
func networkFlag(v any) bool {
	switch v := v.(type) {
	case uint8:
		return v != 0
	case bool:
		// A bool is encoded as a byte.
		return v
	}
	return false
}

// indexValues returns the values 0 through n-1 of a numbered state.
func indexValues(n int) []any {
	values := make([]any, n)
	for i := range values {
		values[i] = int32(i)
	}
	return values
}
