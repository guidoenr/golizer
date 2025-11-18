package render

import (
	"math"
	"sort"
	"sync/atomic"

	"github.com/guidoenr/golizer/internal/params"
)

type patternFunc func(x, y float64, p params.Parameters, t float64) float64

type patternEntry struct {
	fn        patternFunc
	detailMix float64
}

var patternRegistry = map[string]patternEntry{
	"plasma":  {patternPlasma, 0.4},
	"waves":   {patternWaves, 0.4},
	"ripples": {patternRipples, 0.4},
	"nebula":  {patternNebula, 0.4},
	"noise":   {patternNoise, 0.4},
	"bands":   {patternBands, 0.0},
	"strata":  {patternStrata, 0.1},
	"orbits":  {patternOrbits, 0.15},
}

var noiseOctaves atomic.Int32

func init() {
	noiseOctaves.Store(4)
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

func patternBands(x, y float64, p params.Parameters, t float64) float64 {
	freq := math.Max(0.2, p.Frequency*0.35)
	wave := math.Sin((y + t*0.5) * freq)
	mod := math.Sin((x*0.5+t*0.18)*freq*0.8) * 0.6
	return (wave + mod) * 0.7
}

func patternStrata(x, y float64, p params.Parameters, t float64) float64 {
	layer := math.Sin((y*1.5 + t*0.35) * math.Max(1.2, p.Frequency*0.25))
	cross := math.Sin((x*1.1-t*0.22)*1.8) * 0.4
	return layer*0.8 + cross
}

func patternOrbits(x, y float64, p params.Parameters, t float64) float64 {
	r2 := x*x + y*y
	ring := math.Sin(r2*6.0 + t*0.8)
	sweep := math.Sin((x + y + t*0.5) * math.Max(1.0, p.Frequency*0.2))
	return (ring*0.7 + sweep*0.3)
}

func fractalNoise(x, y float64) float64 {
	octaves := int(noiseOctaves.Load())
	if octaves <= 0 {
		octaves = 1
	}
	amp := 0.5
	freq := 1.0
	total := 0.0
	sumAmp := 0.0

	for i := 0; i < octaves; i++ {
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
	xi := int64(x)
	yi := int64(y)
	// Combine coordinates using a variation of SplitMix64 for good distribution without trig.
	n := uint64(xi)<<32 ^ uint64(uint32(yi))
	n = (n ^ (n >> 33)) * 0xff51afd7ed558ccd
	n = (n ^ (n >> 33)) * 0xc4ceb9fe1a85ec53
	n = n ^ (n >> 33)
	return float64(n&0xffffff) / 16777216.0
}

func smoothstep(v float64) float64 {
	return v * v * (3 - 2*v)
}

func lerpFloat(a, b, t float64) float64 {
	return a*(1-t) + b*t
}

func setNoiseProfile(mode qualityMode) {
	switch mode {
	case qualityEco:
		noiseOctaves.Store(1)
	case qualityBalanced:
		noiseOctaves.Store(2)
	default:
		noiseOctaves.Store(3)
	}
}
