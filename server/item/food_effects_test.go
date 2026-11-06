package item

import (
	"fmt"
	"testing"
	"time"

	"github.com/df-mc/dragonfly/server/entity/effect"
)

// TestFood_HarmfulEffects checks food variants whose consumption can apply
// adverse effects, including chance-based effects and cooked alternatives.
func TestFood_HarmfulEffects(t *testing.T) {
	for _, tc := range []struct {
		food    any
		harmful bool
	}{
		{SpiderEye{}, true}, {PoisonousPotato{}, true}, {Pufferfish{}, true},
		{RottenFlesh{}, true}, {Chicken{}, true}, {Chicken{Cooked: true}, false},
		{Bread{}, false}, {Beef{}, false}, {Beef{Cooked: true}, false},
		{GoldenApple{}, false}, {EnchantedApple{}, false}, {HoneyBottle{}, false},
		{DriedKelp{}, false}, {MelonSlice{}, false},
	} {
		t.Run(fmt.Sprintf("%#v", tc.food), func(t *testing.T) {
			food, ok := tc.food.(interface{ HasHarmfulEffects() bool })
			if !ok {
				t.Fatal("food does not expose its possible adverse effects")
			}
			if food.HasHarmfulEffects() != tc.harmful {
				t.Fatalf("harmful effects = %v, want %v", food.HasHarmfulEffects(), tc.harmful)
			}
		})
	}
	// Each registered stew must have an explicit expected classification.
	want := map[uint8]struct {
		harmful  bool
		name     string
		duration time.Duration
	}{
		0: {false, "night_vision", 5 * time.Second}, 1: {false, "jump_boost", 5 * time.Second},
		2: {true, "weakness", 7 * time.Second}, 3: {true, "blindness", 6 * time.Second},
		4: {true, "poison", 11 * time.Second}, 5: {false, "saturation", 300 * time.Millisecond},
		6: {false, "saturation", 300 * time.Millisecond}, 7: {false, "fire_resistance", 3 * time.Second},
		8: {false, "regeneration", 7 * time.Second}, 9: {true, "wither", 7 * time.Second},
		10: {false, "night_vision", 5 * time.Second}, 11: {true, "blindness", 6 * time.Second},
		12: {true, "nausea", 7 * time.Second},
	}
	for _, stew := range StewTypes() {
		t.Run(fmt.Sprintf("stew_%d", stew.Uint8()), func(t *testing.T) {
			expected, known := want[stew.Uint8()]
			if !known {
				t.Fatal("unclassified stew variant")
			}
			food := SuspiciousStew{Type: stew}
			if food.HasHarmfulEffects() != expected.harmful {
				t.Fatal("stew adverse effects do not match its variant")
			}
			consumer := &foodEffectCollector{}
			residue := food.Consume(nil, consumer)
			if consumer.food != 6 || consumer.saturation != 7.2 || residue.Count() != 1 {
				t.Fatal("stew changed its nourishment or bowl residue")
			}
			if _, bowl := residue.Item().(Bowl); !bowl {
				t.Fatal("stew did not return its bowl")
			}
			if len(consumer.effects) != 1 {
				t.Fatalf("stew applied %d effects", len(consumer.effects))
			}
			got := consumer.effects[0]
			if got.Type().Metadata().Name != expected.name || got.Duration() != expected.duration || got.Level() != 1 {
				t.Fatalf("stew effect changed: %s, %v, level%d", got.Type().Metadata().Name, got.Duration(), got.Level())
			}
		})
	}
}

// foodEffectCollector records the existing food and effect consumption contract.
type foodEffectCollector struct {
	Consumer
	food       int
	saturation float64
	effects    []effect.Effect
}

func (c *foodEffectCollector) Saturate(food int, saturation float64) {
	c.food, c.saturation = food, saturation
}
func (c *foodEffectCollector) AddEffect(e effect.Effect) { c.effects = append(c.effects, e) }
