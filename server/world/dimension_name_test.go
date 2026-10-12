package world

import "testing"

type namedTestDimension struct{ Dimension }

// String deliberately disagrees with the registered name.
func (namedTestDimension) String() string { return "Unregistered label" }

// TestDimension_NameUsesTheOwningDefinition covers builtin names, custom
// registrations and IDs that must not inherit DimensionByID's Overworld fallback.
func TestDimension_NameUsesTheOwningDefinition(t *testing.T) {
	registry := newDimensionRegistry(map[int]Dimension{0: Overworld, 1: Nether, 2: End})
	if err := registry.RegisterDimension(1000, "server:custom", namedTestDimension{Overworld}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		id    int
		name  string
		known bool
	}{
		{0, "Overworld", true}, {1, "Nether", true}, {2, "End", true},
		{1000, "server:custom", true}, {-1, "", false}, {3, "", false}, {1001, "", false},
	} {
		if got, ok := registry.Name(test.id); got != test.name || ok != test.known {
			t.Errorf("Name(%d) = %q, %v, want %q, %v", test.id, got, ok, test.name, test.known)
		}
	}
}
