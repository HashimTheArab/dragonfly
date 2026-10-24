package recipe

import (
	"math"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// CraftingPattern describes row-major shaped ingredients, or shapeless ingredients
// when Shape is zero. Match supports only ordinary 2x2 and 3x3 crafting grids.
type CraftingPattern struct {
	// Input is owned by the caller for constructed patterns and by the pattern
	// for decoded patterns. Its items must remain immutable while matching.
	Input []Item
	Shape Shape
	// Output holds the checked single wire result when decoded from a network recipe.
	// It aliases the caller's recipe output slice, including nested stack data. The
	// caller must keep that storage immutable while the pattern is in use. Match
	// does not read Output; caller-created domain patterns may leave it nil.
	Output []protocol.ItemStack
}

// GridConsumption is a checked removal from a zero-based row-major grid slot.
type GridConsumption struct {
	Slot  int
	Count int
}

// valid rejects malformed or unsupported patterns before any matching occurs.
func (p CraftingPattern) valid() bool {
	if len(p.Input) == 0 || len(p.Input) > 9 {
		return false
	}
	if p.Shape != (Shape{}) && (p.Shape.Width() < 1 || p.Shape.Width() > 3 || p.Shape.Height() < 1 || p.Shape.Height() > 3 || p.Shape.Width()*p.Shape.Height() != len(p.Input)) {
		return false
	}
	nonEmpty := false
	for _, in := range p.Input {
		switch in := in.(type) {
		case item.Stack:
			nonEmpty = nonEmpty || !in.Empty()
		case ItemTag:
			if in.Empty() || len(in.items) == 0 {
				return false
			}
			nonEmpty = true
		default:
			return false
		}
	}
	return nonEmpty
}

// Match validates the whole grid and returns removals in slot order without mutating
// it. Shaped recipes may be translated and mirrored horizontally; every other cell
// must be empty. Shapeless recipes require one occupied slot per non-empty ingredient
// and use a complete assignment, including overlapping tags and exact ingredients.
// Counts must suffice for all times crafts; invalid input or a mismatch returns false.
func (p CraftingPattern) Match(grid []item.Stack, times int) ([]GridConsumption, bool) {
	width := 0
	switch len(grid) {
	case 4:
		width = 2
	case 9:
		width = 3
	default:
		return nil, false
	}
	if times < 1 || !p.valid() {
		return nil, false
	}
	for _, in := range p.Input {
		if in.Count() > math.MaxInt/times {
			return nil, false
		}
	}
	if p.Shape == (Shape{}) {
		return p.matchShapeless(grid, times)
	}
	for y := 0; y <= width-p.Shape.Height(); y++ {
		for x := 0; x <= width-p.Shape.Width(); x++ {
			for _, mirror := range []bool{false, true} {
				if used, ok := p.matchShaped(grid, times, width, x, y, mirror); ok {
					return used, true
				}
			}
		}
	}
	return nil, false
}

// matchShaped checks one translation and orientation, including cells outside it.
func (p CraftingPattern) matchShaped(grid []item.Stack, times, width, x, y int, mirror bool) ([]GridConsumption, bool) {
	var used []GridConsumption
	for slot, has := range grid {
		rx, ry := slot%width-x, slot/width-y
		var expected Item = item.Stack{}
		if rx >= 0 && ry >= 0 && rx < p.Shape.Width() && ry < p.Shape.Height() {
			if mirror {
				rx = p.Shape.Width() - 1 - rx
			}
			expected = p.Input[ry*p.Shape.Width()+rx]
		}
		if has.Empty() != expected.Empty() {
			return nil, false
		}
		if expected.Empty() {
			continue
		}
		count := expected.Count() * times
		match, ok := MatchingStacks(has, expected)
		if !ok || !match || has.Count() < count {
			return nil, false
		}
		used = append(used, GridConsumption{Slot: slot, Count: count})
	}
	return used, true
}

// matchShapeless finds a full bipartite matching instead of committing to a greedy
// choice that could leave an exact ingredient without its only compatible slot.
func (p CraftingPattern) matchShapeless(grid []item.Stack, times int) ([]GridConsumption, bool) {
	var ingredients []Item
	occupied := 0
	for _, in := range p.Input {
		if !in.Empty() {
			ingredients = append(ingredients, in)
		}
	}
	for _, has := range grid {
		if !has.Empty() {
			occupied++
		}
	}
	if occupied != len(ingredients) {
		return nil, false
	}
	edges := make([][]int, len(ingredients))
	for i, in := range ingredients {
		for slot, has := range grid {
			if has.Empty() || has.Count() < in.Count()*times {
				continue
			}
			match, ok := MatchingStacks(has, in)
			if !ok {
				return nil, false
			}
			if match {
				edges[i] = append(edges[i], slot)
			}
		}
	}
	assigned := make([]int, len(grid))
	for i := range assigned {
		assigned[i] = -1
	}
	// assign moves prior assignments along augmenting paths, bounded by nine slots.
	var assign func(int, []bool) bool
	assign = func(i int, seen []bool) bool {
		for _, slot := range edges[i] {
			if seen[slot] {
				continue
			}
			seen[slot] = true
			if assigned[slot] == -1 || assign(assigned[slot], seen) {
				assigned[slot] = i
				return true
			}
		}
		return false
	}
	for i := range ingredients {
		if !assign(i, make([]bool, len(grid))) {
			return nil, false
		}
	}
	used := make([]GridConsumption, 0, occupied)
	for slot, i := range assigned {
		if i >= 0 {
			used = append(used, GridConsumption{Slot: slot, Count: ingredients[i].Count() * times})
		}
	}
	return used, true
}
