package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
)

// TestFood_RegisteredConsumablesHaveEffectMetadata requires each registered
// meal to expose its effects, while explicitly excluding non-food drinks.
func TestFood_RegisteredConsumablesHaveEffectMetadata(t *testing.T) {
	for _, it := range world.Items() {
		if _, consumable := it.(item.Consumable); !consumable {
			continue
		}
		switch it.(type) {
		case item.Potion, item.Bucket:
			continue
		}
		if _, food := it.(item.Food); !food {
			name, meta := it.EncodeItem()
			t.Errorf("registered consumable %s:%d lacks food effect metadata", name, meta)
		}
	}
	for _, food := range []item.Food{Carrot{}, Potato{}, SweetBerries{}} {
		if food.HasHarmfulEffects() {
			t.Fatalf("ordinary crop %T reports adverse effects", food)
		}
	}
}
