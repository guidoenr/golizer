package params

import (
	"math"

	"github.com/guidoenr/chroma/go-implementation/internal/analyzer"
)

// Parameters models shader-friendly configuration adapted for CPU rendering.
type Parameters struct {
	Time             float64
	Frequency        float64
	Amplitude        float64
	Speed            float64
	Scale            float64
	ColorShift       float64
	Pattern          string
	ColorMode        string
	Brightness       float64
	Contrast         float64
	Saturation       float64
	Gamma            float64
	Vignette         float64
	VignetteSoftness float64
	GlyphSharpness   float64
	BeatSensitivity  float64
	BassInfluence    float64
	MidInfluence     float64
	TrebleInfluence  float64
	BeatDistortion   float64
	BeatZoom         float64
	DistortAmplitude float64
	NoiseStrength    float64
	NoiseScale       float64
	EffectCooldown   float64
	LastEffectTime   float64
	TerminalBG       [3]uint8
}

// Defaults returns calm defaults similar to the Rust implementation.
func Defaults() Parameters {
	return Parameters{
		Frequency:        6.0,
		Amplitude:        0.4,
		Speed:            0.05,
		Scale:            1.0,
		ColorShift:       0.0,
		Pattern:          "plasma",
		ColorMode:        "chromatic",
		Brightness:       0.6,
		Contrast:         0.8,
		Saturation:       0.9,
		Gamma:            1.0,
		Vignette:         0.25,
		VignetteSoftness: 0.55,
		GlyphSharpness:   1.0,
		BeatSensitivity:  1.0,
		BassInfluence:    0.6,
		MidInfluence:     0.35,
		TrebleInfluence:  0.25,
		BeatDistortion:   0.8,
		BeatZoom:         0.0,
		DistortAmplitude: 0.4,
		NoiseStrength:    0.1,
		NoiseScale:       0.006,
		LastEffectTime:   -100,
	}
}

// UpdateTime advances the internal timer based on frame delta.
func (p *Parameters) UpdateTime(delta float64) {
	p.Time += delta * p.Speed
}

// ApplyFeatures updates parameters based on analyzed audio features.
func (p *Parameters) ApplyFeatures(feat analyzer.Features, delta float64) {
	if feat == (analyzer.Features{}) {
		p.applySilenceDecay(delta)
		return
	}

	energy := maxFloat(0.05, feat.Bass*0.1+feat.Mid*0.3+feat.Treble*0.6)

	bassMultiplier := 1.0 + feat.Bass*p.BassInfluence*0.8
	p.Amplitude = lerp(p.Amplitude, bassMultiplier, 0.5)
	p.NoiseStrength = feat.BeatStrength * (0.3 + feat.Treble*0.7)
	p.DistortAmplitude = lerp(p.DistortAmplitude, 0.35+feat.Bass*0.6, 0.45)
	p.NoiseScale = lerp(p.NoiseScale, 0.004+feat.Mid*0.003, 0.25)

	p.Frequency = lerp(p.Frequency, 8.0*(1.0+feat.Mid*p.MidInfluence*2.0), 0.5)

	baseSpeed := 0.08 + energy*0.9
	trebleBoost := 1.0 + feat.Treble*p.TrebleInfluence*2.5
	targetSpeed := baseSpeed * trebleBoost
	p.Speed = lerp(p.Speed, targetSpeed, 0.55)

	p.ColorShift = math.Mod(p.ColorShift+feat.Treble*0.6, 2*math.Pi)
	p.Gamma = lerp(p.Gamma, 0.9+feat.Mid*0.4, 0.35)
	p.Vignette = lerp(p.Vignette, 0.25+feat.Treble*0.2, 0.25)
	p.GlyphSharpness = lerp(p.GlyphSharpness, 0.9+feat.BeatStrength*0.4, 0.3)

	if feat.IsDrop {
		p.LastEffectTime = p.Time
		p.BeatDistortion = 1.2
		p.BeatZoom = 1.0
		p.DistortAmplitude = 0.95
	} else {
		threshold := 0.18 / maxFloat(0.1, p.BeatSensitivity)
		if feat.BeatStrength > threshold {
			p.LastEffectTime = p.Time
			p.BeatDistortion = 0.85
			p.BeatZoom = 0.7
		}
	}

	trebleBrightness := feat.Treble * 1.5
	beatBoost := feat.BeatStrength * 0.4
	p.Brightness = clamp(p.Brightness*0.6+0.4+feat.Overall+trebleBrightness+beatBoost, 0, 2.2)

	trebleContrast := feat.Treble * 0.8
	p.Contrast = lerp(p.Contrast, 0.6+energy*0.6+trebleContrast, 0.5)

	bassSat := feat.Bass * 0.3
	beatSat := feat.BeatStrength * 0.2
	targetSat := clamp(0.7+bassSat+beatSat, 0.0, 1.5)
	if targetSat > p.Saturation {
		p.Saturation = lerp(p.Saturation, targetSat, 0.7)
	} else {
		p.Saturation = lerp(p.Saturation, targetSat, 0.3)
	}
}

func (p *Parameters) applySilenceDecay(delta float64) {
	decay := math.Pow(0.92, delta*60)
	speedDecay := math.Pow(0.88, delta*60)

	p.Amplitude = p.Amplitude*decay + 0.4*(1-decay)
	p.NoiseStrength *= decay
	p.Frequency = p.Frequency*decay + 6.0*(1-decay)
	p.Speed *= speedDecay
	p.Brightness = p.Brightness*decay + 0.6*(1-decay)
	p.Contrast = p.Contrast*decay + 0.8*(1-decay)
	p.Gamma = lerp(p.Gamma, 1.0, 0.1)
	p.Saturation = lerp(p.Saturation, 0.8, 0.1)
	p.BeatDistortion *= decay
	p.BeatZoom *= decay
	p.DistortAmplitude = lerp(p.DistortAmplitude, 0.4, 0.2)
	p.NoiseScale = lerp(p.NoiseScale, 0.006, 0.3)
	p.GlyphSharpness = lerp(p.GlyphSharpness, 1.0, 0.15)
	p.Vignette = lerp(p.Vignette, 0.25, 0.25)
}

func lerp(current, target, factor float64) float64 {
	return current*(1-factor) + target*factor
}

func clamp(v, minVal, maxVal float64) float64 {
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
