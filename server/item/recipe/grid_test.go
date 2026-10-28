package recipe

import (
	"math"
	"reflect"
	"testing"

	"github.com/df-mc/dragonfly/server/item"
)

// gridStack returns a test stack with an explicit item identity and metadata.
func gridStack(name string, meta int16, count int) item.Stack {
	return item.NewStack(gridTestItem{name, meta}, count)
}

func TestCraftingPattern_ShapedPlacementAndExactEmpties(t *testing.T) {
	a, b := gridStack("a", 0, 2), gridStack("b", 0, 4)
	pattern := CraftingPattern{Input: []Item{a, item.Stack{}, b, a}, Shape: NewShape(2, 2), AssumeSymmetry: true}
	for _, width := range []int{2, 3} {
		for y := 0; y <= width-2; y++ {
			for x := 0; x <= width-2; x++ {
				for _, mirror := range []bool{false, true} {
					grid := make([]item.Stack, width*width)
					for ry := 0; ry < 2; ry++ {
						for rx := 0; rx < 2; rx++ {
							col := rx
							if mirror {
								col = 1 - rx
							}
							grid[(y+ry)*width+x+col] = pattern.Input[ry*2+rx].(item.Stack)
						}
					}
					before := append([]item.Stack(nil), grid...)
					used, ok := pattern.Match(grid, 1)
					if !ok || len(used) != 3 {
						t.Fatalf("placement width=%d x=%d y=%d mirror=%v: %v %v", width, x, y, mirror, used, ok)
					}
					for _, use := range used {
						if use.Count != grid[use.Slot].Count() {
							t.Fatalf("bad removal: %v", use)
						}
					}
					if !reflect.DeepEqual(grid, before) {
						t.Fatal("match mutated grid")
					}
					// Filling the shape's intentional hole must invalidate both orientations.
					for i := range grid {
						if grid[i].Empty() {
							grid[i] = gridStack("unrelated", 0, 1)
						}
					}
					if _, ok := pattern.Match(grid, 1); ok {
						t.Fatal("accepted extra items")
					}
				}
			}
		}
	}
	// Same multiset, different shape.
	if _, ok := pattern.Match([]item.Stack{a, b, a, {}}, 1); ok {
		t.Fatal("accepted wrong shape")
	}
	if _, ok := (CraftingPattern{Input: []Item{a, a, a}, Shape: NewShape(3, 1)}).Match(make([]item.Stack, 4), 1); ok {
		t.Fatal("accepted oversized shape")
	}
}

func TestCraftingPattern_ShapelessOverlappingIngredients(t *testing.T) {
	oak, birch := gridStack("minecraft:oak_planks", 0, 4), gridStack("minecraft:birch_planks", 0, 4)
	exact := gridStack("minecraft:oak_planks", 0, 1)
	tag := NewItemTag("minecraft:planks", 1)
	wildcard := gridStack("minecraft:oak_planks", 0, 1).WithValue("variants", true)
	variant := gridStack("minecraft:oak_planks", 1, 4)
	for _, tt := range []struct {
		name  string
		input []Item
		grid  []item.Stack
		want  []GridConsumption
	}{
		{"tag before exact", []Item{tag, exact}, []item.Stack{oak, birch, {}, {}}, []GridConsumption{{0, 2}, {1, 2}}},
		{"wildcard before exact", []Item{wildcard, exact}, []item.Stack{oak, variant, {}, {}}, []GridConsumption{{0, 2}, {1, 2}}},
		{"tag wildcard exact chain", []Item{tag, wildcard, exact}, []item.Stack{oak, variant, birch, {}}, []GridConsumption{{0, 2}, {1, 2}, {2, 2}}},
		{"count constrained repeated item", []Item{exact, exact.Grow(2)}, []item.Stack{oak.Grow(2), oak.Grow(-2), {}, {}}, []GridConsumption{{0, 6}, {1, 2}}},
		{"empty ingredients ignored", []Item{item.Stack{}, tag, exact}, []item.Stack{{}, birch, {}, oak}, []GridConsumption{{1, 2}, {3, 2}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := CraftingPattern{Input: tt.input}
			got, ok := p.Match(tt.grid, 2)
			if !ok || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v,%v want %v", got, ok, tt.want)
			}
		})
	}
	p := CraftingPattern{Input: []Item{tag, exact}}
	for _, grid := range [][]item.Stack{{oak, {}, {}, {}}, {birch, birch, {}, {}}, {oak, birch, oak, {}}, {oak.Grow(-3), birch, {}, {}}} {
		if _, ok := p.Match(grid, 2); ok {
			t.Fatalf("accepted invalid grid: %v", grid)
		}
	}
	// Repeated ingredients need separate occupied cells, even with enough total items.
	if _, ok := (CraftingPattern{Input: []Item{exact, exact}}).Match([]item.Stack{oak, {}, {}, {}}, 1); ok {
		t.Fatal("combined two ingredient cells")
	}
}

func TestCraftingPattern_RejectsInvalidPatternsAndCounts(t *testing.T) {
	type foreign struct{ Item }
	st := gridStack("a", 0, 1)
	grid := []item.Stack{st, {}, {}, {}}
	for _, p := range []CraftingPattern{
		{}, {Input: []Item{item.Stack{}}}, {Input: []Item{nil}}, {Input: []Item{foreign{}}},
		{Input: []Item{NewItemTag("unknown", 1)}}, {Input: []Item{NewItemTag("minecraft:planks", 0)}},
		{Input: []Item{st}, Shape: NewShape(0, 1)}, {Input: []Item{st}, Shape: NewShape(-1, -1)},
		{Input: []Item{st}, Shape: NewShape(4, 1)}, {Input: []Item{st}, Shape: NewShape(2, 2)},
		{Input: make([]Item, 10)},
	} {
		if _, ok := p.Match(grid, 1); ok {
			t.Fatalf("accepted invalid pattern %+v", p)
		}
	}
	for _, times := range []int{0, -1, 2} {
		if _, ok := (CraftingPattern{Input: []Item{st}}).Match(grid, times); ok {
			t.Fatalf("accepted times=%d", times)
		}
	}
	for _, size := range []int{0, 1, 6, 16} {
		if _, ok := (CraftingPattern{Input: []Item{st}}).Match(make([]item.Stack, size), 1); ok {
			t.Fatalf("accepted grid size=%d", size)
		}
	}
	if _, ok := (CraftingPattern{Input: []Item{st.Grow(1)}}).Match(grid, math.MaxInt); ok {
		t.Fatal("accepted count overflow")
	}
	for _, shape := range []Shape{{}, NewShape(1, 1)} {
		used, ok := (CraftingPattern{Input: []Item{st}, Shape: shape}).Match([]item.Stack{st.Grow(4), {}, {}, {}}, 3)
		if !ok || !reflect.DeepEqual(used, []GridConsumption{{0, 3}}) {
			t.Fatalf("bad multi-craft consumption %v,%v", used, ok)
		}
	}
}
