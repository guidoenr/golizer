package analyzer

// Features describes spectral energy distribution and rhythmic cues extracted from audio.
type Features struct {
	Bass         float64
	Mid          float64
	Treble       float64
	Overall      float64
	BeatStrength float64
	IsDrop       bool
}
