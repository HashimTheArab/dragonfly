package effect

import (
	"image/color"
)

// SlowFalling is a lasting effect that causes the affected entity to fall very
// slowly.
var SlowFalling slowFalling

type slowFalling struct {
	nopLasting
}

// RGBA ...
func (slowFalling) RGBA() color.RGBA {
	return color.RGBA{R: 0xf3, G: 0xcf, B: 0xb9, A: 0xff}
}

// Metadata returns the effect's name and icon.
func (slowFalling) Metadata() Metadata {
	return Metadata{Name: "slow_falling", Icon: "slow_falling_effect"}
}
