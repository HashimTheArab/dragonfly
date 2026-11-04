package customblock

import (
	"reflect"
	"testing"
)

// Traits sort placement_position first, as vanilla lists and applies them.
func TestSortTraitsVanillaOrder(t *testing.T) {
	direction := PlacementDirection{CardinalDirection: true}
	position := PlacementPosition{VerticalHalf: true}
	got, err := SortTraits([]Trait{direction, position})
	if err != nil {
		t.Fatalf("SortTraits: %v", err)
	}
	if want := []Trait{position, direction}; !reflect.DeepEqual(got, want) {
		t.Errorf("SortTraits = %v, want %v", got, want)
	}
}

// SortTraits rejects what vanilla rejects in a pack's traits.
func TestSortTraitsRejectsInvalid(t *testing.T) {
	for name, traits := range map[string][]Trait{
		"duplicate":          {PlacementPosition{BlockFace: true}, PlacementPosition{VerticalHalf: true}},
		"no direction state": {PlacementDirection{}},
		"no position state":  {PlacementPosition{}},
		"offset 45":          {PlacementDirection{CardinalDirection: true, YRotationOffset: 45}},
		"offset -90":         {PlacementDirection{CardinalDirection: true, YRotationOffset: -90}},
		"offset 450":         {PlacementDirection{CardinalDirection: true, YRotationOffset: 450}},
	} {
		if _, err := SortTraits(traits); err == nil {
			t.Errorf("%s: SortTraits returned no error", name)
		}
	}
	for _, offset := range []float64{0, 90, 180, 270, 360} {
		if _, err := SortTraits([]Trait{PlacementDirection{FacingDirection: true, YRotationOffset: offset}}); err != nil {
			t.Errorf("offset %v: %v", offset, err)
		}
	}
}
