package blockinternal

import (
	"github.com/df-mc/dragonfly/server/block/customblock"
	"github.com/df-mc/dragonfly/server/item/category"
	"maps"
	"slices"
)

// molangVersion is the Molang version the client parses the definition's expressions at: the
// latest, which vanilla's own definitions use.
const molangVersion = int32(13)

// ComponentBuilder represents a builder that can be used to construct a block components map to be sent to a client.
type ComponentBuilder struct {
	permutations map[string]map[string]any
	properties   []map[string]any
	traits       []map[string]any
	components   map[string]any
	blockID      int32
	// conditions holds the conditions of permutations in the order they were first added.
	conditions []string

	identifier   string
	menuCategory category.Category
	tags         []string
}

// NewComponentBuilder returns a new component builder with the provided block data, using the provided components map
// as a base.
func NewComponentBuilder(identifier string, components map[string]any, blockID int32) *ComponentBuilder {
	if components == nil {
		components = map[string]any{}
	}
	return &ComponentBuilder{
		permutations: make(map[string]map[string]any),
		components:   components,
		blockID:      blockID,

		identifier:   identifier,
		menuCategory: category.Construction(),
	}
}

// SetTags sets the vanilla block tags of the block, replacing any previously set.
func (builder *ComponentBuilder) SetTags(tags ...string) {
	builder.tags = slices.Clone(tags)
}

// AddProperty adds the provided block property to the builder.
func (builder *ComponentBuilder) AddProperty(name string, values []any) {
	builder.properties = append(builder.properties, map[string]any{
		"name": name,
		"enum": values,
	})
}

// AddTrait adds the provided block trait to the builder.
func (builder *ComponentBuilder) AddTrait(trait customblock.Trait) {
	builder.traits = append(builder.traits, trait.Encode())
}

// AddComponent adds the provided component to the builder. If the component already exists, it will be overwritten.
func (builder *ComponentBuilder) AddComponent(name string, value any) {
	builder.components[name] = value
}

// AddPermutation adds a permutation to the builder. If there is already an existing permutation for the provided
// condition, the new components will be added to the existing permutation.
func (builder *ComponentBuilder) AddPermutation(condition string, components map[string]any) {
	if len(builder.permutations) == 0 {
		// This trigger really does not matter at all, the component just needs to be set for custom block placements to
		// function as expected client-side, when permutations are applied.
		builder.AddComponent("minecraft:on_player_placing", map[string]any{
			"triggerType": "placement_trigger",
		})
	}
	if builder.permutations[condition] == nil {
		builder.permutations[condition] = map[string]any{}
		builder.conditions = append(builder.conditions, condition)
	}
	for key, value := range components {
		builder.permutations[condition][key] = value
	}
}

// SetMenuCategory sets the creative category for the current block.
func (builder *ComponentBuilder) SetMenuCategory(category category.Category) {
	builder.menuCategory = category
}

// Construct constructs the final block components map that is ready to be sent to the client.
func (builder *ComponentBuilder) Construct() map[string]any {
	properties := slices.Clone(builder.properties)
	components := maps.Clone(builder.components)

	result := map[string]any{
		"components":    components,
		"molangVersion": molangVersion,
		"menu_category": map[string]any{
			"category": builder.menuCategory.String(),
			"group":    builder.menuCategory.Group(),
		},
		"vanilla_block_data": map[string]any{
			"block_id": builder.blockID,
		},
	}
	if len(properties) > 0 {
		result["properties"] = properties
	}
	if len(builder.traits) > 0 {
		result["traits"] = slices.Clone(builder.traits)
	}
	if len(builder.tags) > 0 {
		tags := make([]any, 0, len(builder.tags))
		for _, tag := range builder.tags {
			tags = append(tags, tag)
		}
		result["blockTags"] = tags
	}

	if len(builder.conditions) > 0 {
		// The client applies permutations in list order, a later one's components winning where two hold, so they
		// keep the order they were added in.
		permutations := make([]map[string]any, 0, len(builder.conditions))
		for _, condition := range builder.conditions {
			permutations = append(permutations, map[string]any{
				"condition":  condition,
				"components": maps.Clone(builder.permutations[condition]),
			})
		}
		result["permutations"] = permutations
	}
	return result
}
