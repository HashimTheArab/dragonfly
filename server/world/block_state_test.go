package world

import "testing"

func TestUnknownBlockContainerSize(t *testing.T) {
	tests := map[string]struct {
		name string
		data map[string]any
		want int
	}{
		"single trapped chest": {
			name: "minecraft:trapped_chest",
			want: 27,
		},
		"paired trapped chest": {
			name: "minecraft:trapped_chest",
			data: map[string]any{"pairx": int32(2), "pairz": int32(3)},
			want: 54,
		},
		"single copper chest": {
			name: "minecraft:copper_chest",
			want: 27,
		},
		"paired waxed weathered copper chest": {
			name: "minecraft:waxed_weathered_copper_chest",
			data: map[string]any{"pairx": int32(2), "pairz": int32(3)},
			want: 54,
		},
		"unrelated unknown block": {
			name: "minecraft:copper_chestplate",
			want: 0,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			b := unknownBlock{BlockState: BlockState{Name: test.name}, data: test.data}
			if got := b.ContainerSize(); got != test.want {
				t.Fatalf("container size = %d, want %d", got, test.want)
			}
		})
	}
}
