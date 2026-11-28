package item

import (
	"time"

	"github.com/df-mc/dragonfly/server/entity/effect"
)

// StewType represents a type of suspicious stew.
type StewType struct {
	stewType
}

// NightVisionPoppyStew returns suspicious stew night vision effect.
func NightVisionPoppyStew() StewType {
	return StewType{0}
}

// JumpBoostStew returns suspicious stew jump boost effect.
func JumpBoostStew() StewType {
	return StewType{1}
}

// WeaknessStew returns suspicious stew weakness effect.
func WeaknessStew() StewType {
	return StewType{2}
}

// BlindnessBluetStew returns suspicious stew blindness effect.
func BlindnessBluetStew() StewType {
	return StewType{3}
}

// PoisonStew returns suspicious stew poison effect.
func PoisonStew() StewType {
	return StewType{4}
}

// SaturationDandelionStew returns suspicious stew saturation effect.
func SaturationDandelionStew() StewType {
	return StewType{5}
}

// SaturationOrchidStew returns suspicious stew saturation effect.
func SaturationOrchidStew() StewType {
	return StewType{6}
}

// FireResistanceStew returns suspicious stew fire resistance effect.
func FireResistanceStew() StewType {
	return StewType{7}
}

// RegenerationStew returns suspicious stew regeneration effect.
func RegenerationStew() StewType {
	return StewType{8}
}

// WitherStew returns suspicious stew wither effect.
func WitherStew() StewType {
	return StewType{9}
}

// NightVisionTorchflowerStew returns suspicious stew night vision effect.
func NightVisionTorchflowerStew() StewType {
	return StewType{10}
}

// BlindnessEyeblossomStew returns suspicious stew blindness effect.
func BlindnessEyeblossomStew() StewType {
	return StewType{11}
}

// NauseaStew returns suspicious stew nausea effect.
func NauseaStew() StewType {
	return StewType{12}
}

// StewTypes ...
func StewTypes() []StewType {
	return []StewType{NightVisionPoppyStew(), JumpBoostStew(), WeaknessStew(), BlindnessBluetStew(), PoisonStew(), SaturationDandelionStew(), SaturationOrchidStew(), FireResistanceStew(), RegenerationStew(), WitherStew(), NightVisionTorchflowerStew(), BlindnessEyeblossomStew(), NauseaStew()}
}

type stewType uint8

// Uint8 returns the stew as a uint8.
func (s stewType) Uint8() uint8 {
	return uint8(s)
}

// Effects returns suspicious stew effects.
func (s stewType) Effects() []effect.Effect {
	typ, duration, _ := s.effectInfo()
	return []effect.Effect{effect.New(typ, 1, duration)}
}

// effectInfo keeps a stew's effect, duration and adverse classification together.
func (s stewType) effectInfo() (effect.LastingType, time.Duration, bool) {
	switch s.Uint8() {
	case 0, 10:
		return effect.NightVision, time.Second * 5, false
	case 1:
		return effect.JumpBoost, time.Second * 5, false
	case 2:
		return effect.Weakness, time.Second * 7, true
	case 3, 11:
		return effect.Blindness, time.Second * 6, true
	case 4:
		return effect.Poison, time.Second * 11, true
	case 5, 6:
		return effect.Saturation, time.Second * 3 / 10, false
	case 7:
		return effect.FireResistance, time.Second * 3, false
	case 8:
		return effect.Regeneration, time.Second * 7, false
	case 9:
		return effect.Wither, time.Second * 7, true
	case 12:
		return effect.Nausea, time.Second * 7, true
	default:
		panic("should never happen")
	}
}
