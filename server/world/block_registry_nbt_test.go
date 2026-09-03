package world

import "testing"

// TestBlockByNBTState_ResolvesLooselyTypedProperties resolves a state whose
// boolean properties were written as integers, which is how structure files and
// other third-party sources spell them. Property values hash by type, so such a
// state misses the palette entirely and the block silently fails to resolve.
func TestBlockByNBTState_ResolvesLooselyTypedProperties(t *testing.T) {
	registry := NewBlockRegistry()
	registry.RegisterBlockState(BlockState{
		Name:       "dragonfly:loose_state",
		Properties: map[string]any{"open_bit": true, "direction": int32(3)},
	})
	registry.Finalize()

	loose := map[string]any{"open_bit": int32(1), "direction": int32(3)}
	if _, ok := registry.BlockByName("dragonfly:loose_state", loose); ok {
		t.Fatal("BlockByName resolved an int-typed boolean; the coercion is no longer needed")
	}
	if _, ok := registry.BlockByNBTState("dragonfly:loose_state", loose); !ok {
		t.Fatalf("BlockByNBTState did not resolve %v", loose)
	}
}

// TestBlockByNBTState_KeepsNumericPropertiesNumeric refuses a state whose
// numeric property does not match, rather than flattening it to a boolean and
// resolving to the wrong direction.
func TestBlockByNBTState_KeepsNumericPropertiesNumeric(t *testing.T) {
	registry := NewBlockRegistry()
	registry.RegisterBlockState(BlockState{
		Name:       "dragonfly:loose_state",
		Properties: map[string]any{"open_bit": true, "direction": int32(3)},
	})
	registry.Finalize()

	wrongDirection := map[string]any{"open_bit": int32(1), "direction": int32(1)}
	if _, ok := registry.BlockByNBTState("dragonfly:loose_state", wrongDirection); ok {
		t.Fatal("BlockByNBTState resolved a state whose direction does not exist")
	}
	if _, ok := registry.BlockByNBTState("dragonfly:missing", map[string]any{}); ok {
		t.Fatal("BlockByNBTState resolved an unregistered block name")
	}
}

// TestBlockByNBTState_DropsPropertiesThePaletteDoesNotDeclare resolves a state
// carrying properties no registered state holds. A fence derives its arms from
// its neighbours rather than storing them, so a structure file that does record
// them describes a state the palette has no entry for.
func TestBlockByNBTState_DropsPropertiesThePaletteDoesNotDeclare(t *testing.T) {
	registry := NewBlockRegistry()
	registry.RegisterBlockState(BlockState{Name: "dragonfly:derived_state"})
	registry.Finalize()

	connections := map[string]any{"north": int32(1), "east": int32(0)}
	if _, ok := registry.BlockByName("dragonfly:derived_state", connections); ok {
		t.Fatal("BlockByName resolved undeclared properties; the fallback is no longer needed")
	}
	if _, ok := registry.BlockByNBTState("dragonfly:derived_state", connections); !ok {
		t.Fatalf("BlockByNBTState did not resolve %v", connections)
	}
}

// TestBlockByNBTState_PrefersTheDeclaredState keeps a state that matches every
// declared property, so dropping the unknown ones never picks a different one.
func TestBlockByNBTState_PrefersTheDeclaredState(t *testing.T) {
	registry := NewBlockRegistry()
	registry.RegisterBlockState(BlockState{
		Name:       "dragonfly:mixed_state",
		Properties: map[string]any{"direction": int32(2)},
	})
	registry.RegisterBlockState(BlockState{
		Name:       "dragonfly:mixed_state",
		Properties: map[string]any{"direction": int32(3)},
	})
	registry.Finalize()

	block, ok := registry.BlockByNBTState("dragonfly:mixed_state", map[string]any{
		"direction": int32(3), "has_book": int32(1),
	})
	if !ok {
		t.Fatal("BlockByNBTState did not resolve a state with one unknown property")
	}
	_, properties := block.EncodeBlock()
	if properties["direction"] != int32(3) {
		t.Fatalf("resolved direction = %#v, want int32(3)", properties["direction"])
	}
}

// TestCoerceStateValue covers the per-type conversion the lookup relies on.
func TestCoerceStateValue(t *testing.T) {
	for _, test := range []struct {
		name           string
		value, declare any
		want           any
	}{
		{name: "int to bool", value: int32(1), declare: false, want: true},
		{name: "zero to bool", value: int32(0), declare: false, want: false},
		{name: "byte to bool", value: uint8(1), declare: false, want: true},
		{name: "int stays int", value: int32(3), declare: int32(0), want: int32(3)},
		{name: "int to byte", value: int32(2), declare: uint8(0), want: uint8(2)},
		{name: "string untouched", value: "north", declare: "south", want: "north"},
		{name: "unknown property untouched", value: int32(7), declare: nil, want: int32(7)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := coerceStateValue(test.value, test.declare); got != test.want {
				t.Fatalf("coerceStateValue(%#v, %#v) = %#v, want %#v", test.value, test.declare, got, test.want)
			}
		})
	}
}
