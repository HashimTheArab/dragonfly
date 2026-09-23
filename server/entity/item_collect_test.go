package entity

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

// TestItemCollectBox pins the vanilla pickup range: an item reaches one block
// sideways and half a block vertically past its own box for a collector.
func TestItemCollectBox(t *testing.T) {
	player := cube.Box(-0.3, 0, -0.3, 0.3, 1.8, 0.3)
	item := mgl64.Vec3{0, 0, 0}
	for _, test := range []struct {
		name string
		feet mgl64.Vec3
		want bool
	}{
		{"standing on it", mgl64.Vec3{0, 0, 0}, true},
		{"1.4 blocks to the side", mgl64.Vec3{1.4, 0, 0}, true},
		{"1.5 blocks to the side", mgl64.Vec3{1.5, 0, 0}, false},
		{"diagonal within both axes", mgl64.Vec3{1.4, 0, 1.4}, true},
		{"feet 0.7 above it", mgl64.Vec3{0, 0.7, 0}, true},
		{"feet 0.8 above it", mgl64.Vec3{0, 0.8, 0}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ItemCollectBox(item).IntersectsWith(player.Translate(test.feet)); got != test.want {
				t.Fatalf("collector at %v in range = %v, want %v", test.feet, got, test.want)
			}
		})
	}
}
