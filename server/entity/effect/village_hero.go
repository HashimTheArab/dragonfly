package effect

// VillageHero describes a client effect whose server-side behaviour is not implemented.
var VillageHero villageHero

type villageHero struct{}

// Metadata returns the effect's name and icon.
func (villageHero) Metadata() Metadata {
	return Metadata{Name: "village_hero", Icon: "village_hero_effect"}
}
