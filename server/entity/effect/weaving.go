package effect

// Weaving describes a client effect whose server-side behaviour is not implemented.
var Weaving weaving

type weaving struct{}

// Metadata returns the effect's name and icon.
func (weaving) Metadata() Metadata {
	return Metadata{Name: "weaving", Icon: "weaving_effect"}
}
