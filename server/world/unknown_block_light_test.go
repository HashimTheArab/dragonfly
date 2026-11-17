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

// The registry's light tables agree with what each block without an
// implementation reports about its own state, including a state a server
// adds after the registry is finalized.
func TestUnknownBlockLightMatchesRegistryTables(t *testing.T) {
	br := NewBlockRegistry().(*BasicBlockRegistry)
	br.Finalize()
	if _, err := br.addCustomBlockState(BlockState{Name: "server:lamp"}, nil); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for rid, b := range br.Blocks() {
		u, ok := b.(unknownBlock)
		if !ok {
			continue
		}
		checked++
		if got, want := br.LightBlock(uint32(rid)), u.LightEmissionLevel(); got != want {
			t.Errorf("%s %v: table light %d, block reports %d", u.Name, u.Properties, got, want)
		}
		if got, want := br.FilteringBlock(uint32(rid)), u.LightDiffusionLevel(); got != want {
			t.Errorf("%s %v: table filter %d, block reports %d", u.Name, u.Properties, got, want)
		}
	}
	if checked == 0 {
		t.Fatal("no blocks without an implementation were checked")
	}
}
