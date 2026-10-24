package effect

// BadOmen describes a client effect whose server-side behaviour is not implemented.
var BadOmen badOmen

type badOmen struct{}

// Metadata returns the effect's name and icon.
func (badOmen) Metadata() Metadata {
	return Metadata{Name: "bad_omen", Icon: "bad_omen_effect"}
}
