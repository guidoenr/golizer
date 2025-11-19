package params

import (
	"math"

	"github.com/guidoenr/golizer/internal/analyzer"
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
		Amplitude:        0.0,
		Speed:            0.05,
		Scale:            1.0,
		ColorShift:       0.0,
		Pattern:          "pulse",
		ColorMode:        "chromatic",
		Brightness:       0.0,
		Contrast:         0.8,
		Saturation:       0.9,
		Gamma:            1.0,
		Vignette:         0.25,
		VignetteSoftness: 0.55,
		GlyphSharpness:   1.0,
		BeatSensitivity:  1.2,
		BassInfluence:    0.9,
		MidInfluence:     0.15,
		TrebleInfluence:  0.08,
		BeatDistortion:   0.0,
		BeatZoom:         0.0,
		DistortAmplitude: 0.0,
		NoiseStrength:    0.0,
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

	// Energía enfocada en bass (kicks/low end)
	energy := maxFloat(0.05, feat.Bass*0.7+feat.Mid*0.2+feat.Treble*0.1)

	// Amplitud reacciona principalmente al bass - RÁPIDO
	bassMultiplier := 1.0 + feat.Bass*p.BassInfluence*1.5
	p.Amplitude = lerp(p.Amplitude, bassMultiplier, 0.85)
	
	// Noise/distortion responde a beats, no tanto a treble - INSTANTÁNEO
	p.NoiseStrength = feat.BeatStrength * (0.5 + feat.Bass*0.8)
	p.DistortAmplitude = lerp(p.DistortAmplitude, 0.4+feat.Bass*0.9, 0.75)
	p.NoiseScale = lerp(p.NoiseScale, 0.004+feat.Bass*0.003, 0.6)

	// Frecuencia sube suavemente con bass
	p.Frequency = lerp(p.Frequency, 6.0*(1.0+feat.Bass*0.6+feat.Mid*p.MidInfluence), 0.6)

	// Velocidad más estable, enfocada en bass
	baseSpeed := 0.08 + energy*0.8
	trebleBoost := 1.0 + feat.Treble*p.TrebleInfluence
	targetSpeed := baseSpeed * trebleBoost
	p.Speed = lerp(p.Speed, targetSpeed, 0.6)

	// Color shift más suave
	p.ColorShift = math.Mod(p.ColorShift+feat.Bass*0.3+feat.Treble*0.15, 2*math.Pi)
	p.Gamma = lerp(p.Gamma, 0.9+feat.Bass*0.3, 0.3)
	p.Vignette = lerp(p.Vignette, 0.25+feat.BeatStrength*0.15, 0.2)
	p.GlyphSharpness = lerp(p.GlyphSharpness, 0.9+feat.BeatStrength*0.5, 0.35)

	// Reacción a drops y kicks
	if feat.IsDrop {
		p.LastEffectTime = p.Time
		p.BeatDistortion = 1.5
		p.BeatZoom = 1.2
		p.DistortAmplitude = 1.0
	} else {
		threshold := 0.16 / maxFloat(0.1, p.BeatSensitivity)
		if feat.BeatStrength > threshold {
			p.LastEffectTime = p.Time
			p.BeatDistortion = 1.0
			p.BeatZoom = 0.8
		}
	}

	// Brillo enfocado en bass y beats - MUY RÁPIDO y EXPLOSIVO
	bassBrightness := feat.Bass * 1.2
	beatBoost := feat.BeatStrength * 0.8
	targetBrightness := clamp(feat.Overall*0.8+bassBrightness+beatBoost, 0, 2.5)
	p.Brightness = lerp(p.Brightness, targetBrightness, 0.85) // Sube y baja rapidísimo

	// Contraste responde principalmente a bass
	bassContrast := feat.Bass * 0.8
	p.Contrast = lerp(p.Contrast, 0.8+energy*0.6+bassContrast, 0.7)

	// Saturación con bass y beats - RÁPIDA
	bassSat := feat.Bass * 0.7
	beatSat := feat.BeatStrength * 0.5
	targetSat := clamp(0.9+bassSat+beatSat, 0.0, 1.6)
	if targetSat > p.Saturation {
		p.Saturation = lerp(p.Saturation, targetSat, 0.85)
	} else {
		p.Saturation = lerp(p.Saturation, targetSat, 0.7)
	}
}

func (p *Parameters) applySilenceDecay(delta float64) {
	// Decay MUY rápido - vuelve a negro casi instantáneamente
	fastDecay := math.Pow(0.75, delta*60)     // Muy rápido
	superFastDecay := math.Pow(0.5, delta*60) // Super rápido para brightness

	p.Amplitude = p.Amplitude * fastDecay
	p.NoiseStrength *= superFastDecay
	p.Frequency = p.Frequency*fastDecay + 6.0*(1-fastDecay)
	p.Speed = p.Speed * fastDecay
	p.Brightness = p.Brightness * superFastDecay // Se apaga rapidísimo
	p.Contrast = p.Contrast*fastDecay + 0.8*(1-fastDecay)
	p.Gamma = lerp(p.Gamma, 1.0, 0.3)
	p.Saturation = lerp(p.Saturation, 0.8, 0.3)
	p.BeatDistortion *= superFastDecay
	p.BeatZoom *= superFastDecay
	p.DistortAmplitude *= superFastDecay
	p.NoiseScale = lerp(p.NoiseScale, 0.006, 0.4)
	p.GlyphSharpness = lerp(p.GlyphSharpness, 1.0, 0.3)
	p.Vignette = lerp(p.Vignette, 0.25, 0.4)
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
