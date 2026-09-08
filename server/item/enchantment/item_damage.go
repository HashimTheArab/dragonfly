package enchantment

import "github.com/df-mc/dragonfly/server/item"

// DamageItem applies Unbreaking and then durability loss, including unbreakable
// items and the broken-item result. Callers apply game-mode and event policy first.
func DamageItem(s item.Stack, damage int) item.Stack {
	if damage <= 0 {
		return s
	}
	if e, ok := s.Enchantment(Unbreaking); ok {
		damage = Unbreaking.Reduce(s.Item(), e.Level(), damage)
	}
	return s.Damage(damage)
}
