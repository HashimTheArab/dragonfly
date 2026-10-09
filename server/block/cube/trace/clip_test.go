package trace_test

import (
	"math"
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/cube/trace"
	"github.com/go-gl/mathgl/mgl64"
)

func TestClipBlockRayStart(t *testing.T) {
	bounds := cube.Range{-64, 319}
	for _, test := range []struct {
		name             string
		start, end, want mgl64.Vec3
		ok               bool
	}{
		{name: "inside unchanged", start: mgl64.Vec3{1, 300, 2}, end: mgl64.Vec3{1, 330, 2}, want: mgl64.Vec3{1, 300, 2}, ok: true},
		{name: "above looking down", start: mgl64.Vec3{1, 321.62, 2}, end: mgl64.Vec3{1, 316.62, 2}, want: mgl64.Vec3{1, math.Nextafter(320, -64), 2}, ok: true},
		{name: "below looking up", start: mgl64.Vec3{1, -66, 2}, end: mgl64.Vec3{5, -62, 6}, want: mgl64.Vec3{3, -64, 4}, ok: true},
		{name: "lower horizontal", start: mgl64.Vec3{1, -64, 2}, end: mgl64.Vec3{5, -64, 6}, want: mgl64.Vec3{1, -64, 2}, ok: true},
		{name: "tiny boundary crossing", start: mgl64.Vec3{1, 320.000001, 2}, end: mgl64.Vec3{1, 319.999999, 2}, want: mgl64.Vec3{1, math.Nextafter(320, -64), 2}, ok: true},
		{name: "above looking away", start: mgl64.Vec3{1, 321, 2}, end: mgl64.Vec3{1, 325, 2}},
		{name: "above horizontal", start: mgl64.Vec3{1, 321, 2}, end: mgl64.Vec3{5, 321, 6}},
		{name: "never enters", start: mgl64.Vec3{1, 322, 2}, end: mgl64.Vec3{1, 321, 2}},
		{name: "touches exclusive ceiling", start: mgl64.Vec3{1, 322, 2}, end: mgl64.Vec3{1, 320, 2}},
		{name: "nonfinite", start: mgl64.Vec3{math.NaN(), 322, 2}, end: mgl64.Vec3{1, 318, 2}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := trace.ClipBlockRayStart(bounds, test.start, test.end)
			if ok != test.ok || ok && got != test.want {
				t.Fatalf("clipped start = %v, %v; want %v, %v", got, ok, test.want, test.ok)
			}
		})
	}
}
