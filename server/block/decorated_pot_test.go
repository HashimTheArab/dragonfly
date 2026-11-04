package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/item"
)

func TestDecoratedPotDecodeNBTIgnoresExtraSherds(t *testing.T) {
	pot := (DecoratedPot{}).DecodeNBT(map[string]any{
		"sherds": []any{
			"minecraft:brick",
			"minecraft:brick",
			"minecraft:brick",
			"minecraft:brick",
			"minecraft:brick",
		},
	}).(DecoratedPot)

	for i, decoration := range pot.Decorations {
		if _, ok := decoration.(item.Brick); !ok {
			t.Fatalf("decoration %v was %T, expected item.Brick", i, decoration)
		}
	}
}

func TestDecoratedPotDecodeNBT_IgnoresInvalidSherds(t *testing.T) {
	pot := (DecoratedPot{}).DecodeNBT(map[string]any{
		"sherds": []any{
			"minecraft:brick",
			"minecraft:air",
			"example:missing_sherd",
			int32(1),
		},
	}).(DecoratedPot)

	if _, ok := pot.Decorations[0].(item.Brick); !ok {
		t.Fatalf("valid decoration was %T, expected item.Brick", pot.Decorations[0])
	}
	for i, decoration := range pot.Decorations[1:] {
		if decoration != nil {
			t.Fatalf("invalid decoration %d was retained as %T", i+1, decoration)
		}
	}
}
