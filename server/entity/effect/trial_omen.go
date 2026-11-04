package effect

// TrialOmen describes a client effect whose server-side behaviour is not implemented.
var TrialOmen trialOmen

type trialOmen struct{}

// Metadata returns the effect's name and icon.
func (trialOmen) Metadata() Metadata {
	return Metadata{Name: "trial_omen", Icon: "trial_omen_effect"}
}
