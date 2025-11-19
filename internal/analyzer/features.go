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

// GateFeatures applies a simple noise floor so weak signals are ignored.
func GateFeatures(f Features, floor float64) Features {
	if floor <= 0 {
		return f
	}
	gate := func(v float64) float64 {
		if v <= floor {
			return 0
		}
		return clampFloat((v-floor)/(1.0-floor), 0, 1)
	}

	f.Bass = gate(f.Bass)
	f.Mid = gate(f.Mid)
	f.Treble = gate(f.Treble)
	f.Overall = gate(f.Overall)
	if f.BeatStrength <= floor {
		f.BeatStrength = 0
	} else {
		f.BeatStrength = clampFloat((f.BeatStrength-floor)/(1.0-floor), 0, 1)
	}
	if f.Overall == 0 && f.Bass == 0 && f.Mid == 0 && f.Treble == 0 {
		f.IsDrop = false
	}
	return f
}

func clampFloat(v, minVal, maxVal float64) float64 {
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}
