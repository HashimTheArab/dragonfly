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
		{name: "ordinary item", item: Stick{}, want: UseContinuationUnclassified},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyUseContinuation(tt.item); got != tt.want {
				t.Fatalf("ClassifyUseContinuation() = %d, want %d", got, tt.want)
			}
		})
	}
}
