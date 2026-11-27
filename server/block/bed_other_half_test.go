package block

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
)

func TestBedOtherHalf(t *testing.T) {
	pos := cube.Pos{3, 64, -2}
	for _, facing := range cube.Directions() {
		foot := Bed{Facing: facing}
		head := Bed{Facing: facing, Head: true}
		headPos := foot.OtherHalf(pos)
		if headPos != pos.Side(facing.Face()) {
			t.Fatalf("foot facing %v: other half at %v, want %v", facing, headPos, pos.Side(facing.Face()))
		}
		if got := head.OtherHalf(headPos); got != pos {
			t.Fatalf("head facing %v: other half at %v, want the foot at %v", facing, got, pos)
		}
	}
}
