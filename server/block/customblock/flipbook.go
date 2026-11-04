package customblock

// Flipbook animates a block texture held as a vertical strip of square frames, as an entry of
// a resource pack's textures/flipbook_textures.json.
type Flipbook struct {
	// TicksPerFrame is the number of ticks each frame is shown for. Zero leaves the client's
	// default of one tick.
	TicksPerFrame int
	// Frames is the order the frames of the strip are shown in, by their index from the top. If
	// empty, every frame is shown once from top to bottom.
	Frames []int
	// NoBlend stops the client blending each frame into the next while showing it, which it
	// does by default.
	NoBlend bool
}

// Encode returns the flipbook as an entry of textures/flipbook_textures.json for the terrain
// texture with the atlas key passed, whose image is at texturePath in the pack.
func (f Flipbook) Encode(atlasTile, texturePath string) map[string]any {
	entry := map[string]any{
		"flipbook_texture": texturePath,
		"atlas_tile":       atlasTile,
	}
	if f.NoBlend {
		entry["blend_frames"] = false
	}
	if f.TicksPerFrame > 0 {
		entry["ticks_per_frame"] = f.TicksPerFrame
	}
	if len(f.Frames) > 0 {
		entry["frames"] = f.Frames
	}
	return entry
}
