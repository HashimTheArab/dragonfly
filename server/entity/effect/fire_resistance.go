package effect

import (
	"image/color"
)

// FireResistance is a lasting effect that grants immunity to fire & lava damage.
var FireResistance fireResistance

type fireResistance struct {
	nopLasting
}

// RGBA ...
func (fireResistance) RGBA() color.RGBA {
	return color.RGBA{R: 0xff, G: 0x99, B: 0x00, A: 0xff}
}

// Metadata returns the effect's name and icon.
func (fireResistance) Metadata() Metadata {
	return Metadata{Name: "fire_resistance", Icon: "fire_resistance_effect"}
}
