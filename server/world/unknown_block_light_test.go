package world

import "testing"

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
