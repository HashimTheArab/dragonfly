package item

import "math"

// Repeatable is a stack that can be counted and grown, which is all RepeatStacks needs of
// it. Stack implements it; so does any type wrapping a Stack.
type Repeatable[T any] interface {
	// Count returns the number of items in the stack.
	Count() int
	// MaxCount returns the highest count a single stack of this item may hold.
	MaxCount() int
	// Grow returns the stack with its count raised by the amount passed.
	Grow(int) T
}

// RepeatStacks multiplies the count of every stack passed by repetitions, splitting a
// result that would exceed an item's max count across as many stacks as it takes.
func RepeatStacks[T Repeatable[T]](items []T, repetitions int) []T {
	output := make([]T, 0, len(items))
	for _, o := range items {
		count, maxCount := o.Count(), o.MaxCount()
		total := count * repetitions

		stacks := int(math.Ceil(float64(total) / float64(maxCount)))
		for range stacks {
			inc := min(total, maxCount)
			total -= inc

			output = append(output, o.Grow(inc-count))
		}
	}
	return output
}
