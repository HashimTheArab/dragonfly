package block

import (
	"slices"
	"strings"

	shareditem "github.com/bedrock-mc/protocolgen/generated/data/item"
)

// CopperChestNames returns the copper chest oxidation and wax variants from the
// shared vanilla catalog. No gameplay implementation is implied by these names.
func CopperChestNames() []string {
	var names []string
	for _, entry := range shareditem.RuntimeItems() {
		if strings.HasSuffix(entry.Name, "copper_chest") {
			names = append(names, entry.Name)
		}
	}
	slices.Sort(names)
	return names
}
