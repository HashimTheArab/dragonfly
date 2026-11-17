package block

import (
	"slices"
	"strings"

	shareditem "github.com/cinnabar-mc/protocolgen/generated/data/item"
)

// CopperChestNames returns the copper chest oxidation and wax variants from the
// shared vanilla catalog. No gameplay implementation is implied by these names.
func CopperChestNames() []string { return catalogNames("copper_chest") }

// catalogNames returns the sorted names in the shared vanilla catalog that end
// in suffix, such as every wood variant of one block.
func catalogNames(suffix string) []string {
	var names []string
	for _, entry := range shareditem.RuntimeItems() {
		if strings.HasSuffix(entry.Name, suffix) {
			names = append(names, entry.Name)
		}
	}
	slices.Sort(names)
	return names
}
