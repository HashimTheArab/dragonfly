package entity

import (
	vanilla "github.com/bedrock-mc/bedrock-data/generated/entity"
	"github.com/df-mc/dragonfly/server/world"
)

// CowType is the persistent type of a cow.
var CowType = animalType{&animalDefinition{data: vanilla.Cow, spawning: []animalSpawnRule{
	{weight: 8, minimum: 2, maximum: 3, tags: []string{"animal"}},
}}}

// PigType is the persistent type of a pig.
var PigType = animalType{&animalDefinition{data: vanilla.Pig, spawning: []animalSpawnRule{
	{weight: 10, minimum: 1, maximum: 3, tags: []string{"animal", "cherry_grove"}},
}}}

// SheepType is the persistent type of a sheep.
var SheepType = animalType{&animalDefinition{data: vanilla.Sheep, spawning: []animalSpawnRule{
	{weight: 12, minimum: 2, maximum: 3, tags: []string{"animal"}},
	{weight: 2, minimum: 2, maximum: 4, tags: []string{"meadow", "cherry_grove"}},
}}}

// ChickenType is the persistent type of a chicken.
var ChickenType = animalType{&animalDefinition{data: vanilla.Chicken, spawning: []animalSpawnRule{
	{weight: 10, minimum: 2, maximum: 4, tags: []string{"animal"}},
}}}

// animalTypes lists species supported by the shared living runtime. A vanilla
// data entry alone does not implement a species' behaviour. Registration and
// population both use this list so they cannot select different species.
var animalTypes = []animalType{CowType, PigType, SheepType, ChickenType}

// animalDefinition combines shared vanilla values with the spawn rules this
// runtime supports. Definitions are immutable; mutable state belongs to an actor.
type animalDefinition struct {
	data     vanilla.Entity
	spawning []animalSpawnRule
}

type animalType struct{ definition *animalDefinition }

type animalSpawnRule struct {
	weight, minimum, maximum int
	tags                     []string
}

// animalEntityTypes adapts the supported species for the world's registry.
func animalEntityTypes() []world.EntityType {
	types := make([]world.EntityType, 0, len(animalTypes))
	for _, species := range animalTypes {
		types = append(types, species)
	}
	return types
}
