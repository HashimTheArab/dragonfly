package block

import (
	"slices"
	"testing"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

// TestCropBreakInfo_PossibleDropsDoNotSample proves an optional outcome is known
// even when the count callback would omit it, without invoking that callback.
func TestCropBreakInfo_PossibleDropsDoNotSample(t *testing.T) {
	calls, count := 0, 0
	info := cropBreakInfo(0, nothingEffective, cropDrop{item: item.PoisonousPotato{}, count: func([]item.Enchantment) int {
		calls++
		return count
	}})
	if calls != 0 {
		t.Fatalf("constructing possible drops sampled counts %d times", calls)
	}
	if got := possibleDropNames(info); !slices.Equal(got, []string{"minecraft:poisonous_potato"}) {
		t.Fatalf("possible drops = %v, want poisonous potato before any count is drawn", got)
	}
	if got := info.Drops(item.ToolNone{}, nil); len(got) != 0 || calls != 1 {
		t.Fatalf("zero-count harvest = %v, count calls = %d; want no items and one count call", got, calls)
	}
	count = 1
	drops := info.Drops(item.ToolNone{}, nil)
	if len(drops) != 1 || calls != 2 {
		t.Fatalf("positive-count harvest = %v, count calls = %d; want one item and two count calls", drops, calls)
	}
	name, _ := drops[0].Item().EncodeItem()
	possibleName, _ := info.PossibleDrops[0].EncodeItem()
	if name != possibleName || drops[0].Count() != 1 {
		t.Fatalf("harvest = %v, want the declared possible item with count one", drops)
	}
}

// TestReplantable_PossibleDrops checks maturity-sensitive produce and rare loot
// through deterministic metadata, without relying on a lucky random harvest.
func TestReplantable_PossibleDrops(t *testing.T) {
	wheat, carrot, potato, beetroot := WheatSeeds{}, Carrot{}, Potato{}, BeetrootSeeds{}
	wheat.Growth, carrot.Growth, potato.Growth, beetroot.Growth = 7, 7, 7, 7
	tests := []struct {
		name  string
		plant Replantable
		want  []string
	}{
		{"mature wheat", wheat, []string{"minecraft:wheat", "minecraft:wheat_seeds"}},
		{"young wheat", WheatSeeds{}, []string{"minecraft:wheat_seeds"}},
		{"mature beetroot", beetroot, []string{"minecraft:beetroot", "minecraft:beetroot_seeds"}},
		{"young beetroot", BeetrootSeeds{}, []string{"minecraft:beetroot_seeds"}},
		{"mature potato", potato, []string{"minecraft:poisonous_potato", "minecraft:potato"}},
		{"young potato", Potato{}, []string{"minecraft:potato"}},
		{"mature carrot", carrot, []string{"minecraft:carrot"}},
		{"young carrot", Carrot{}, []string{"minecraft:carrot"}},
		{"mature nether wart", NetherWart{Age: 3}, []string{"minecraft:nether_wart"}},
		{"young nether wart", NetherWart{Age: 2}, []string{"minecraft:nether_wart"}},
		{"mature cocoa", CocoaBean{Age: 2}, []string{"minecraft:cocoa_beans"}},
		{"young cocoa", CocoaBean{Age: 1}, []string{"minecraft:cocoa_beans"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := test.plant.BreakInfo()
			if got := possibleDropNames(info); !slices.Equal(got, test.want) {
				t.Fatalf("possible drops = %v, want %v", got, test.want)
			}
		})
	}
}

// TestReplantable_AllStatesHavePossibleDrops guards new crop registrations and
// ensures the metadata covers each state and the drops it actually produces.
func TestReplantable_AllStatesHavePossibleDrops(t *testing.T) {
	registry := world.NewBlockRegistry()
	registry.Finalize()
	plants := 0
	for _, b := range registry.Blocks() {
		plant, ok := b.(Replantable)
		if !ok {
			continue
		}
		plants++
		info := plant.BreakInfo()
		if len(info.PossibleDrops) == 0 {
			t.Fatalf("%#v has no possible-drop metadata", plant)
		}
		possible := possibleDropNames(info)
		for _, drop := range info.Drops(item.ToolNone{}, nil) {
			name, _ := drop.Item().EncodeItem()
			if !slices.Contains(possible, name) {
				t.Fatalf("%#v produced %q outside possible drops %v", plant, name, possible)
			}
		}
	}
	if plants == 0 {
		t.Fatal("no replantable plants registered")
	}
}

// TestCropBreakInfo_EmptyAndUnavailable distinguishes known empty outcomes from
// blocks whose possible-drop metadata has not been supplied.
func TestCropBreakInfo_EmptyAndUnavailable(t *testing.T) {
	if got := cropBreakInfo(0, nothingEffective).PossibleDrops; got == nil || len(got) != 0 {
		t.Fatalf("empty definition metadata = %v, want a non-nil empty slice", got)
	}
	if got := (Stone{}).BreakInfo().PossibleDrops; got != nil {
		t.Fatalf("unavailable metadata = %v, want nil", got)
	}
}

// possibleDropNames returns sorted canonical identities for an outcome set.
func possibleDropNames(info BreakInfo) []string {
	names := make([]string, len(info.PossibleDrops))
	for i, possible := range info.PossibleDrops {
		names[i], _ = possible.EncodeItem()
	}
	slices.Sort(names)
	return names
}
