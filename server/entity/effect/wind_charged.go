package effect

// WindCharged describes a client effect whose server-side behaviour is not implemented.
var WindCharged windCharged

type windCharged struct{}

// Metadata returns the effect's name and icon.
func (windCharged) Metadata() Metadata {
	return Metadata{Name: "wind_charged", Icon: "wind_charged_effect"}
}
