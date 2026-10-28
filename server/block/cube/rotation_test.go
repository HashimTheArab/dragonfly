package cube

import (
	"math"
	"testing"

	"github.com/go-gl/mathgl/mgl64"
)

func TestRotation_Vec3VerticalPitch(t *testing.T) {
	for _, test := range []struct {
		name  string
		rot   Rotation
		wantY float64
	}{
		{name: "straight down", rot: Rotation{0, 90}, wantY: -1},
		{name: "straight up", rot: Rotation{0, -90}, wantY: 1},
		{name: "straight down after a turn", rot: Rotation{270, 90}, wantY: -1},
		{name: "level", rot: Rotation{0, 0}, wantY: 0},
		{name: "overflown past down", rot: Rotation{0, 100}, wantY: math.Sin(mgl64.DegToRad(80))},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.rot.Vec3()[1]; math.Abs(got-test.wantY) > 1e-9 {
				t.Fatalf("Vec3().Y = %v, want %v", got, test.wantY)
			}
		})
	}
}

func TestRotation_VerticalOppositeAndNeg(t *testing.T) {
	down := Rotation{0, 90}
	if got := down.Opposite().Vec3()[1]; math.Abs(got-1) > 1e-9 {
		t.Fatalf("Opposite of straight down has Y = %v, want 1", got)
	}
	if got := down.Neg().Pitch(); got != -90 {
		t.Fatalf("Neg of straight down has pitch %v, want -90", got)
	}
	if got := (Rotation{0, 80}).Add(Rotation{0, 10}).Pitch(); got != 90 {
		t.Fatalf("Add reaching straight down has pitch %v, want 90", got)
	}
}
