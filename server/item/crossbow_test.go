package item

import (
	"testing"

	"github.com/df-mc/dragonfly/server/world"
)

func TestClassifyUseContinuation(t *testing.T) {
	for _, tt := range []struct {
		name string
		item world.Item
		want UseContinuation
	}{
		{name: "consumable", item: Apple{}, want: UseContinuationContinued},
		{name: "releasable", item: Bow{}, want: UseContinuationContinued},
		{name: "unloaded crossbow", item: Crossbow{}, want: UseContinuationContinued},
		{name: "loaded crossbow", item: Crossbow{Item: NewStack(Arrow{}, 1)}, want: UseContinuationInstant},
		{name: "unloaded crossbow pointer", item: &Crossbow{}, want: UseContinuationContinued},
		{name: "loaded crossbow pointer", item: &Crossbow{Item: NewStack(Arrow{}, 1)}, want: UseContinuationInstant},
		{name: "glass bottle", item: GlassBottle{}, want: UseContinuationInstant},
		{name: "glass bottle pointer", item: &GlassBottle{}, want: UseContinuationInstant},
		{name: "bowl", item: Bowl{}, want: UseContinuationInstant},
		{name: "bowl pointer", item: &Bowl{}, want: UseContinuationInstant},
		{name: "empty bucket", item: Bucket{}, want: UseContinuationInstant},
		{name: "empty bucket pointer", item: &Bucket{}, want: UseContinuationInstant},
		{name: "milk bucket", item: Bucket{Content: MilkBucketContent()}, want: UseContinuationContinued},
		{name: "milk bucket pointer", item: &Bucket{Content: MilkBucketContent()}, want: UseContinuationContinued},
		{name: "nil bucket pointer", item: (*Bucket)(nil), want: UseContinuationUnclassified},
		{name: "ordinary item", item: Stick{}, want: UseContinuationUnclassified},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyUseContinuation(tt.item); got != tt.want {
				t.Fatalf("ClassifyUseContinuation() = %d, want %d", got, tt.want)
			}
		})
	}
}
