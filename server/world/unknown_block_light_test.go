package world

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world/chunk"
)

// A block without an implementation gives off the light of its own state:
// a lit copper bulb and a lava cauldron shine, an unlit bulb and an empty
// cauldron do not.
func TestUnknownBlockLightFollowsState(t *testing.T) {
	DefaultBlockRegistry.Finalize()
	levels := map[string]map[bool]bool{}
	for _, b := range DefaultBlockRegistry.Blocks() {
		u, ok := b.(unknownBlock)
		if !ok {
			continue
		}
		var on bool
		switch u.Name {
		case "minecraft:copper_bulb":
			on = u.Properties["lit"] == uint8(1)
		case "minecraft:cauldron":
			on = u.Properties["cauldron_liquid"] == "lava" && u.Properties["fill_level"] != int32(0)
		default:
			continue
		}
		if levels[u.Name] == nil {
			levels[u.Name] = map[bool]bool{}
		}
		levels[u.Name][on] = true
		if lit := u.LightEmissionLevel() == 15; lit != on {
			t.Errorf("%s %v gives off light %d", u.Name, u.Properties, u.LightEmissionLevel())
		}
	}
	for _, name := range []string{"minecraft:copper_bulb", "minecraft:cauldron"} {
		if !levels[name][true] || !levels[name][false] {
			t.Errorf("%s states seen = %v; want both lit and unlit ones", name, levels[name])
		}
	}
}

// HighestLightBlockerBelow finds the highest full light blocker under a
// height, as HighestLightBlocker does for the whole column, skipping blocks
// that let light through.
func TestHighestLightBlockerBelow(t *testing.T) {
	DefaultBlockRegistry.Finalize()
	c := chunk.New(DefaultBlockRegistry, Overworld.Range())
	stone, glass := stateRuntimeIDForTest(t, "minecraft:stone"), stateRuntimeIDForTest(t, "minecraft:glass")
	c.SetBlock(1, 10, 1, 0, stone)
	c.SetBlock(1, 100, 1, 0, stone)
	c.SetBlock(1, 150, 1, 0, glass)
	minY := int16(Overworld.Range()[0])
	for _, tc := range []struct{ below, want int16 }{
		{320, 100}, {101, 100}, {100, 10}, {11, 10}, {10, minY}, {500, 100},
	} {
		if got := c.HighestLightBlockerBelow(1, 1, tc.below); got != tc.want {
			t.Errorf("HighestLightBlockerBelow(%d) = %d, want %d", tc.below, got, tc.want)
		}
	}
	if got := c.HighestLightBlocker(1, 1); got != 100 {
		t.Errorf("HighestLightBlocker() = %d, want 100", got)
	}
	if got := c.HighestLightBlocker(2, 2); got != minY {
		t.Errorf("empty column HighestLightBlocker() = %d, want %d", got, minY)
	}
}

// stateRuntimeIDForTest returns the runtime ID of the first state named name.
func stateRuntimeIDForTest(t *testing.T, name string) uint32 {
	t.Helper()
	for rid, b := range DefaultBlockRegistry.Blocks() {
		if n, _ := b.EncodeBlock(); n == name {
			return uint32(rid)
		}
	}
	t.Fatalf("no state named %s", name)
	return 0
}
