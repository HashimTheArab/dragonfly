package effect

// Metadata describes an effect independently of its server-side implementation.
type Metadata struct {
	// Name is the stable, lower snake_case identifier, such as "mining_fatigue".
	// It is independent of the player's language and has no namespace prefix.
	Name string
	// Icon is a UI texture name without the "textures/ui/" prefix or extension.
	// Effects that share a visual may use the same icon.
	Icon string
}

// MetadataByID returns the metadata registered with id, including effects that
// have no server-side implementation. The bool is false for unknown effects.
func MetadataByID(id int) (Metadata, bool) {
	effect, ok := effects[id]
	return effect.metadata, ok
}

// AllMetadata returns a copy of the registered effect metadata keyed by ID.
func AllMetadata() map[int]Metadata {
	metadata := make(map[int]Metadata, len(effects))
	for id, effect := range effects {
		metadata[id] = effect.metadata
	}
	return metadata
}
