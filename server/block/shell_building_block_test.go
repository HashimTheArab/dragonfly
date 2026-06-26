package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world"
)

func TestShellBuildingBlocksRegisterForUnknownPaletteStates(t *testing.T) {
	world.DefaultBlockRegistry.Finalize()

	b, ok := world.BlockByName("minecraft:acacia_button", map[string]any{
		"button_pressed_bit": uint8(0),
		"facing_direction":   int32(0),
	})
	if !ok {
		t.Fatal("acacia button state was not found in the block registry")
	}
	if _, ok := b.(shellBuildingBlock); !ok {
		t.Fatalf("acacia button state = %T, want shellBuildingBlock", b)
	}
}
