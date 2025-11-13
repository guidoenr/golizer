package render

import (
	"math"
	"sort"

	"github.com/guidoenr/chroma/go-implementation/internal/params"
)

type patternFunc func(x, y float64, p params.Parameters, t float64) float64

var patternRegistry = map[string]patternFunc{
	"plasma":  patternPlasma,
	"waves":   patternWaves,
	"ripples": patternRipples,
	"nebula":  patternNebula,
	"noise":   patternNoise,
}

// PatternNames returns the available pattern identifiers.
func PatternNames() []string {
	names := make([]string, 0, len(patternRegistry))
	for name := range patternRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func patternPlasma(x, y float64, p params.Parameters, t float64) float64 {
	v1 := math.Sin((x*3.4 + t*1.2) * 0.9)
	v2 := math.Sin((y*4.1 - t*0.7) * 1.1)
	v3 := math.Sin((x+y)*2.3 + t*1.7)
	return (v1 + v2 + v3) / 3.0
}

func patternWaves(x, y float64, p params.Parameters, t float64) float64 {
	freq := p.Frequency * 0.6
	v := math.Sin((x+t*0.8)*freq) * math.Cos((y-t*0.5)*freq*1.1)
	return v
}

func patternRipples(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Hypot(x, y)
	theta := math.Atan2(y, x)
	return math.Sin(r*p.Frequency*1.6 - t*2.2 + math.Sin(theta*3+t)*0.5)
}

func patternNebula(x, y float64, p params.Parameters, t float64) float64 {
	base := patternPlasma(x*0.8, y*0.8, p, t)
	swirl := math.Sin((x-y)*1.5 + t*0.9)
	noise := fractalNoise(x*1.2+t*0.1, y*1.2-t*0.15)
	return (base*0.6 + swirl*0.2 + noise*0.6) / 1.0
}

func patternNoise(x, y float64, p params.Parameters, t float64) float64 {
	scale := math.Max(0.001, p.NoiseScale*60.0)
	return fractalNoise((x+p.ColorShift)*scale+t*0.2, (y-p.ColorShift)*scale-t*0.18)
}

func fractalNoise(x, y float64) float64 {
	amp := 0.5
	freq := 1.0
	total := 0.0
	sumAmp := 0.0

	for i := 0; i < 4; i++ {
		total += valueNoise2(x*freq, y*freq) * amp
		sumAmp += amp
		amp *= 0.5
		freq *= 2.0
	}

	if sumAmp == 0 {
		return 0
	}
	return (total/sumAmp)*2.0 - 1.0
}

func valueNoise2(x, y float64) float64 {
	x0 := math.Floor(x)
	y0 := math.Floor(y)
	x1 := x0 + 1.0
	y1 := y0 + 1.0

	sx := smoothstep(x - x0)
	sy := smoothstep(y - y0)

	n00 := hash2(x0, y0)
	n10 := hash2(x1, y0)
	n01 := hash2(x0, y1)
	n11 := hash2(x1, y1)

	ix0 := lerpFloat(n00, n10, sx)
	ix1 := lerpFloat(n01, n11, sx)

	return lerpFloat(ix0, ix1, sy)
}

func hash2(x, y float64) float64 {
	return frac(math.Sin(x*127.1+y*311.7) * 43758.5453123)
}

func smoothstep(v float64) float64 {
	return v * v * (3 - 2*v)
}

func lerpFloat(a, b, t float64) float64 {
	return a*(1-t) + b*t
}

func frac(v float64) float64 {
	return v - math.Floor(v)
}
