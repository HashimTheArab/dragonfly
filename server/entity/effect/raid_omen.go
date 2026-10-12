package effect

// RaidOmen describes a client effect whose server-side behaviour is not implemented.
var RaidOmen raidOmen

type raidOmen struct{}

// Metadata returns the effect's name and icon.
func (raidOmen) Metadata() Metadata {
	return Metadata{Name: "raid_omen", Icon: "raid_omen_effect"}
}
