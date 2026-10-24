package recipe

import "github.com/df-mc/dragonfly/server/item"

// MatchingStacks reports whether has satisfies expected. ok is false when either side is
// not a recipe item type, which a caller decoding recipes from an untrusted source has to
// handle rather than read as a mismatch.
func MatchingStacks(has, expected Item) (match, ok bool) {
	switch has.(type) {
	case item.Stack, ItemTag:
	default:
		return false, false
	}
	switch expected.(type) {
	case item.Stack, ItemTag:
	default:
		return false, false
	}
	if has.Empty() || expected.Empty() {
		return has.Empty() == expected.Empty(), true
	}
	switch expected := expected.(type) {
	case item.Stack:
		switch has := has.(type) {
		case ItemTag:
			name, _ := expected.Item().EncodeItem()
			return has.Contains(name), true
		case item.Stack:
			_, variants := expected.Value("variants")
			if !variants {
				return has.Comparable(expected), true
			}
			nameOne, _ := has.Item().EncodeItem()
			nameTwo, _ := expected.Item().EncodeItem()
			return nameOne == nameTwo, true
		}
	case ItemTag:
		switch has := has.(type) {
		case item.Stack:
			name, _ := has.Item().EncodeItem()
			return expected.Contains(name), true
		case ItemTag:
			return has.Tag() == expected.Tag(), true
		}
	}
	return false, false
}
