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
	"dots":      {patternDots, 0.0},
	"flash":     {patternFlash, 0.0},
	"grid":      {patternGrid, 0.0},
	"spark":     {patternSpark, 0.1},
	"pulse":     {patternPulse, 0.0},
	"scatter":   {patternScatter, 0.0},
	"beam":      {patternBeam, 0.0},
	"ripple":    {patternRipple, 0.1},
	"strobe":    {patternStrobe, 0.0},
	"particle":  {patternParticle, 0.1},
	"laser":     {patternLaser, 0.0},
	"waves":     {patternWaves, 0.1},
	"orbit":     {patternOrbit, 0.0},
	"explosion": {patternExplosion, 0.1},
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

// Dots - puntos aleatorios que aparecen con el beat
func patternDots(x, y float64, p params.Parameters, t float64) float64 {
	cellX := math.Floor(x * 3.0)
	cellY := math.Floor(y * 3.0)
	noise := hash2(cellX, cellY)
	timing := math.Sin((noise*6.28 + t*p.Frequency*0.5))
	beat := p.BeatDistortion * 3.0
	return math.Max(0, timing+beat-0.8) * 5.0
}

// Flash - destellos intensos en el centro con el beat
func patternFlash(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Hypot(x, y)
	flash := 1.0 - r
	beat := p.BeatDistortion * 2.0
	pulse := math.Sin(t*p.Frequency*2.0) * 0.3
	intensity := (flash + beat + pulse) * 2.0
	return math.Max(0, intensity-1.0)
}

// Grid - grid minimal que aparece con el audio
func patternGrid(x, y float64, p params.Parameters, t float64) float64 {
	gridX := math.Abs(math.Sin(x*p.Frequency*0.8 + t*0.5))
	gridY := math.Abs(math.Sin(y*p.Frequency*0.8 - t*0.3))
	lines := math.Max(gridX, gridY)
	beat := p.Amplitude * 0.8
	return math.Pow(lines, 10.0) * (1.0 + beat)
}

// Spark - chispas que explotan desde el centro
func patternSpark(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Hypot(x, y)
	angle := math.Atan2(y, x)
	rays := math.Abs(math.Sin(angle*5.0 + t*2.0))
	falloff := 1.0 / (1.0 + r*2.0)
	beat := p.BeatDistortion * 1.5
	return rays * falloff * (0.5 + beat)
}

// Pulse - pulso concéntrico minimal
func patternPulse(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Hypot(x, y)
	ring := math.Abs(math.Sin((r-t)*p.Frequency*2.0))
	beat := p.BeatDistortion * 2.0
	return math.Pow(ring, 5.0) * (1.0 + beat)
}

// Scatter - partículas dispersas
func patternScatter(x, y float64, p params.Parameters, t float64) float64 {
	cellX := math.Floor(x*5.0 + math.Sin(t)*2.0)
	cellY := math.Floor(y*5.0 + math.Cos(t*0.8)*2.0)
	noise := hash2(cellX, cellY)
	threshold := 0.9 - p.Amplitude*0.3
	if noise > threshold {
		return (noise - threshold) * 10.0
	}
	return 0
}

// Beam - rayos verticales que reaccionan al audio
func patternBeam(x, y float64, p params.Parameters, t float64) float64 {
	beamPos := math.Sin(t*p.Frequency*0.3) * 0.8
	dist := math.Abs(x - beamPos)
	beam := 1.0 / (1.0 + dist*10.0)
	beat := p.Amplitude * 1.5
	return math.Pow(beam, 3.0) * beat
}

// Ripple - ondas desde puntos aleatorios
func patternRipple(x, y float64, p params.Parameters, t float64) float64 {
	centers := []struct{ cx, cy float64 }{
		{math.Sin(t * 0.3), math.Cos(t * 0.4)},
		{math.Sin(t*0.5 + 1.0), math.Cos(t*0.6 - 1.0)},
	}
	value := 0.0
	for _, c := range centers {
		r := math.Hypot(x-c.cx, y-c.cy)
		ripple := math.Sin(r*p.Frequency*3.0 - t*3.0)
		value += ripple / (1.0 + r)
	}
	return value * p.Amplitude
}

// Strobe - efecto estroboscópico
func patternStrobe(x, y float64, p params.Parameters, t float64) float64 {
	strobe := math.Floor(math.Sin(t*p.Frequency*4.0+p.BeatDistortion*6.28) + 0.5)
	r := math.Hypot(x, y)
	vignette := 1.0 - r*0.5
	return strobe * vignette
}

// Particle - sistema de partículas minimal
func patternParticle(x, y float64, p params.Parameters, t float64) float64 {
	particles := 0.0
	for i := 0.0; i < 8.0; i++ {
		angle := (i / 8.0) * 6.28 + t
		speed := 0.3 + p.Amplitude*0.4
		px := math.Sin(angle) * t * speed
		py := math.Cos(angle) * t * speed
		px = math.Mod(px+2.0, 4.0) - 2.0
		py = math.Mod(py+2.0, 4.0) - 2.0
		dist := math.Hypot(x-px, y-py)
		particles += 1.0 / (1.0 + dist*20.0)
	}
	return particles * 2.0
}

// Laser - líneas laser que cruzan la pantalla
func patternLaser(x, y float64, p params.Parameters, t float64) float64 {
	angle := t * p.Frequency * 0.5
	lineY := x*math.Sin(angle) + y*math.Cos(angle)
	laser := 1.0 / (1.0 + math.Abs(lineY)*50.0)
	beat := p.BeatDistortion * 2.0
	return laser * (0.5 + beat)
}

// Waves - ondas minimalistas
func patternWaves(x, y float64, p params.Parameters, t float64) float64 {
	wave1 := math.Sin(x*p.Frequency*0.6 + t)
	wave2 := math.Sin(y*p.Frequency*0.6 - t*0.7)
	combined := (wave1 + wave2) * 0.5
	return math.Pow(math.Abs(combined), 3.0) * p.Amplitude * 2.0
}

// Orbit - órbitas circulares minimal
func patternOrbit(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Hypot(x, y)
	angle := math.Atan2(y, x)
	orbit := math.Sin(angle*3.0 - t*2.0 + r*2.0)
	ring := 1.0 / (1.0 + math.Abs(r-0.5)*10.0)
	return orbit * ring * p.Amplitude * 2.0
}

// Explosion - explosión desde el centro
func patternExplosion(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Hypot(x, y)
	explosion := math.Sin(r*p.Frequency*2.0 - t*3.0)
	beat := p.BeatDistortion * 3.0
	intensity := math.Exp(-r) * (0.3 + beat)
	return explosion * intensity * 2.0
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
		noiseOctaves.Store(4)
	}
}
