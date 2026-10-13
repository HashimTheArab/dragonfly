package block

import (
	"slices"
	"testing"

	"github.com/df-mc/dragonfly/server/world"
)

func TestLiquidFromLayers_PrecedenceAndUnavailable(t *testing.T) {
	water := Water{Depth: 8}
	lava := Lava{Depth: 8}
	for _, test := range []struct {
		name    string
		blocks  [2]world.Block
		known   [2]bool
		want    world.Liquid
		found   bool
		visited []uint8
	}{
		{name: "primary water", blocks: [2]world.Block{water, lava}, known: [2]bool{true, true}, want: water, found: true, visited: []uint8{0}},
		{name: "primary avoids unknown secondary", blocks: [2]world.Block{water, nil}, known: [2]bool{true, false}, want: water, found: true, visited: []uint8{0}},
		{name: "secondary water", blocks: [2]world.Block{Stone{}, water}, known: [2]bool{true, true}, want: water, found: true, visited: []uint8{0, 1}},
		{name: "secondary lava", blocks: [2]world.Block{Air{}, lava}, known: [2]bool{true, true}, want: lava, found: true, visited: []uint8{0, 1}},
		{name: "dry", blocks: [2]world.Block{Stone{}, Air{}}, known: [2]bool{true, true}, visited: []uint8{0, 1}},
		{name: "unknown primary", blocks: [2]world.Block{nil, water}, known: [2]bool{false, true}, visited: []uint8{0}},
		{name: "unknown secondary", blocks: [2]world.Block{Stone{}, nil}, known: [2]bool{true, false}, visited: []uint8{0, 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var visited []uint8
			got, found := world.LiquidFromLayers(func(layer uint8) (world.Block, bool) {
				visited = append(visited, layer)
				return test.blocks[layer], test.known[layer]
			})
			if got != test.want || found != test.found || !slices.Equal(visited, test.visited) {
				t.Fatalf("liquid = %#v, %v; reads=%v; want %#v, %v; reads=%v", got, found, visited, test.want, test.found, test.visited)
			}
		})
	}
}
