package block

import (
	"image/color"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
)

// WithNBT reads a sign's data as DecodeNBT does, both sides and the legacy
// single-side format, and returns the sign without allocating.
func TestSign_WithNBTMatchesDecodeNBT(t *testing.T) {
	base := Sign{Wood: SpruceWood(), Attach: WallAttachment(cube.North)}
	written := Sign{
		Wood:   SpruceWood(),
		Attach: WallAttachment(cube.North),
		Front:  SignText{Text: "front", BaseColour: color.RGBA{R: 10, G: 20, B: 30, A: 255}, Glowing: true, Owner: "owner"},
		Back:   SignText{Text: "back", BaseColour: color.RGBA{A: 255}},
		Waxed:  true,
	}
	for name, data := range map[string]map[string]any{
		"Sides":  written.EncodeNBT(),
		"Blank":  base.EncodeNBT(),
		"Legacy": {"Text": "old", "SignTextColor": int32(-1), "IgnoreLighting": uint8(1), "TextIgnoreLegacyBugResolved": uint8(1)},
		"Empty":  {},
	} {
		if got, want := base.WithNBT(data), base.DecodeNBT(data).(Sign); got != want {
			t.Errorf("%s: WithNBT() = %+v, want %+v", name, got, want)
		}
		if allocs := testing.AllocsPerRun(10, func() { _ = base.WithNBT(data) }); allocs != 0 {
			t.Errorf("%s: WithNBT() allocated %v times, want none", name, allocs)
		}
	}
}
