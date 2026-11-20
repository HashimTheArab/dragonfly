package world

import "github.com/cinnabar-mc/protocolgen/generated/data/block"

// unknownBlockProperties holds the values used for blocks without an implementation.
type unknownBlockProperties struct {
	friction            float64
	dampening, emission uint8
}

var unknownBlockProps = vanillaBlockProperties()

// newUnknownBlock returns the block for state s without an implementation.
// It carries the light values of s itself, since a block such as a copper
// bulb or a lava cauldron gives off light in some states only.
func newUnknownBlock(s BlockState) unknownBlock {
	b := unknownBlock{BlockState: s}
	hash, _ := networkBlockHash(s.Name, s.Properties, nil)
	// A custom state can share a vanilla state's hash, so the name must match too.
	if state, ok := block.StateByHash(hash); ok && vanillaStateNamed(state, s.Name) {
		if values, ok := block.PropertiesAt(state.Properties); ok {
			b.light = stateLight{emission: values.LightEmission, dampening: values.LightDampening, known: true}
		}
	}
	return b
}

// stateLight is the light one state gives off and filters.
type stateLight struct {
	emission, dampening uint8
	// known is false for a state the shared data lacks.
	known bool
}

// vanillaBlockProperties keeps the first state's friction and the lowest light
// values for each block name. The light values stand in only for a state the
// shared data lacks, as the block's resting value.
func vanillaBlockProperties() map[string]unknownBlockProperties {
	properties := make(map[string]unknownBlockProperties)
	for i := range block.StateCount() {
		state, _ := block.StateAt(i)
		definition, _ := block.BlockAt(state.Block)
		values, _ := block.PropertiesAt(state.Properties)
		previous, ok := properties[definition.Name]
		if !ok {
			properties[definition.Name] = unknownBlockProperties{
				friction: float64(values.Friction), dampening: values.LightDampening, emission: values.LightEmission,
			}
			continue
		}
		previous.dampening = min(previous.dampening, values.LightDampening)
		previous.emission = min(previous.emission, values.LightEmission)
		properties[definition.Name] = previous
	}
	return properties
}

// vanillaStateNamed reports whether the vanilla state belongs to the block named.
func vanillaStateNamed(state block.State, name string) bool {
	b, ok := block.BlockAt(state.Block)
	return ok && b.Name == name
}
