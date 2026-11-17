package block

// DaylightDetector supplies block identity and actor metadata. Gameplay behaviour and
// the block-actor NBT codec are not implemented.
type DaylightDetector struct{}

// EncodeBlock returns the block identifier without selecting a gameplay state.
func (DaylightDetector) EncodeBlock() (string, map[string]any) {
	return "minecraft:daylight_detector", nil
}

// BlockActorID returns the Bedrock NBT identifier for DaylightDetector.
func (DaylightDetector) BlockActorID() string { return "DaylightDetector" }

// BlockActorAliases returns the inverted daylight detector, which shares the actor.
func (DaylightDetector) BlockActorAliases() []string {
	return []string{"minecraft:daylight_detector_inverted"}
}
