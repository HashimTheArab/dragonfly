package entity

import (
	"math"
	"testing"

	"github.com/df-mc/dragonfly/server/world"
)

func TestAnimalDefinitionsSupportLivingRuntime(t *testing.T) {
	names := make(map[string]bool)
	for _, species := range animalTypes {
		t.Run(species.EncodeEntity(), func(t *testing.T) {
			data := species.definition.data
			if names[data.Name] {
				t.Fatal("duplicate species definition")
			}
			names[data.Name] = true
			if !data.CollisionBox.Present || !data.Health.Present || !data.Health.Initial.Present ||
				!data.Health.Maximum.Present || !data.Movement.Present ||
				data.Health.Initial.Ranged || data.Movement.Ranged {
				t.Fatal("species needs fixed collision, health and movement values")
			}
			for _, value := range []float32{data.CollisionBox.Width, data.CollisionBox.Height, data.Health.Initial.Value, data.Health.Maximum.Value, data.Movement.Value} {
				if value <= 0 || math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
					t.Fatalf("invalid living default: %v", value)
				}
			}
			if data.Health.Initial.Value > data.Health.Maximum.Value {
				t.Fatal("initial health exceeds maximum health")
			}
			if registered, ok := DefaultRegistry.Lookup(data.Name); !ok || registered != species {
				t.Fatal("spawnable species is missing from the default registry")
			}
			for _, rule := range species.definition.spawning {
				if rule.weight <= 0 || rule.minimum <= 0 || rule.maximum < rule.minimum || len(rule.tags) == 0 {
					t.Fatalf("invalid spawn rule: %+v", rule)
				}
			}
		})
	}
}

func TestAnimalRuntimeUsesSpeciesData(t *testing.T) {
	definition := *CowType.definition
	definition.data.CollisionBox.Width, definition.data.CollisionBox.Height = 2, 3
	definition.data.Health.Initial.Value, definition.data.Health.Maximum.Value = 6, 12
	definition.data.Movement.Value = .125
	species := animalType{&definition}
	w := world.Config{Synchronous: true}.New()
	defer w.Close()
	w.Do(func(tx *world.Tx) {
		a := tx.AddEntity((world.EntitySpawnOpts{}).New(species, species)).(*Animal)
		if a.Health() != 6 || a.MaxHealth() != 12 || a.Speed() != .125 {
			t.Fatalf("living attributes ignored species data: health=%v maximum=%v speed=%v", a.Health(), a.MaxHealth(), a.Speed())
		}
		if box := species.BBox(a); box.Width() != 2 || box.Height() != 3 {
			t.Fatalf("collision ignored species data: %v", box)
		}
	})
}
