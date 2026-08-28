package item

import "testing"

func TestCrossbow_StartsCharge(t *testing.T) {
	tests := []struct {
		name string
		bow  Crossbow
		want bool
	}{
		{name: "unloaded", bow: Crossbow{}, want: true},
		{name: "loaded", bow: Crossbow{Item: NewStack(Arrow{}, 1)}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bow.StartsCharge(); got != tt.want {
				t.Fatalf("StartsCharge() = %t, want %t", got, tt.want)
			}
		})
	}
}
