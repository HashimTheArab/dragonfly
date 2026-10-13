package effect

// Oozing describes a client effect whose server-side behaviour is not implemented.
var Oozing oozing

type oozing struct{}

// Metadata returns the effect's name and icon.
func (oozing) Metadata() Metadata {
	return Metadata{Name: "oozing", Icon: "oozing_effect"}
}
