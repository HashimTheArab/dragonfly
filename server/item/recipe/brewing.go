package recipe

import (
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

// BrewingStep is one registered transformation of a potion by a reagent.
type BrewingStep struct {
	Input, Reagent, Output world.Item
}

// BrewingSteps finds the shortest registered brewing sequence from input to output.
// The caller supplies the available recipes, allowing a server-specific catalog.
// Container changes retain the potion variant through the same lookup as Perform.
func BrewingSteps(input, output world.Item, available []Recipe) ([]BrewingStep, bool) {
	if input == nil || output == nil {
		return nil, false
	}
	type identity struct {
		name string
		meta int16
	}
	key := func(it world.Item) identity { name, meta := it.EncodeItem(); return identity{name, meta} }
	type node struct {
		it    world.Item
		steps []BrewingStep
	}
	queue := []node{{it: input}}
	seen := map[identity]bool{key(input): true}
	var reagents []world.Item
	index := make(map[string]Recipe)
	reagentSeen := map[identity]bool{}
	for _, r := range available {
		switch r.(type) {
		case Potion, PotionContainerChange:
		default:
			continue
		}
		inputs := r.Input()
		if len(inputs) != 2 {
			continue
		}
		first, ok := inputs[0].(item.Stack)
		if !ok {
			continue
		}
		stack, ok := inputs[1].(item.Stack)
		if !ok || first.Empty() || stack.Empty() {
			continue
		}
		_, containerChange := r.(PotionContainerChange)
		index[hashItems([]world.Item{first.Item(), stack.Item()}, !containerChange)] = r
		if reagentSeen[key(stack.Item())] {
			continue
		}
		reagentSeen[key(stack.Item())] = true
		reagents = append(reagents, stack.Item())
	}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		if key(current.it) == key(output) {
			return current.steps, true
		}
		for _, reagent := range reagents {
			result, ok := performBrewing(index, current.it, reagent)
			if !ok || len(result) != 1 || result[0].Empty() || seen[key(result[0].Item())] {
				continue
			}
			next := result[0].Item()
			seen[key(next)] = true
			steps := append(append([]BrewingStep(nil), current.steps...), BrewingStep{current.it, reagent, next})
			queue = append(queue, node{next, steps})
		}
	}
	return nil, false
}
