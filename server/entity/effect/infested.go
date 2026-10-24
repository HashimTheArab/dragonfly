package effect

// Infested describes a client effect whose server-side behaviour is not implemented.
var Infested infested

type infested struct{}

// Metadata returns the effect's name and icon.
func (infested) Metadata() Metadata {
	return Metadata{Name: "infested", Icon: "infested_effect"}
}
