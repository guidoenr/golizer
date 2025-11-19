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
	timing := noise + p.BeatDistortion*2.0
	if timing > 0.8 {
		return (timing - 0.8) * 5.0
	}
	return 0
}

// Flash - destellos intensos en el centro con el beat
func patternFlash(x, y float64, p params.Parameters, t float64) float64 {
	r := x*x + y*y // más rápido que Hypot
	flash := 1.0 - r
	beat := p.BeatDistortion * 2.0
	intensity := (flash + beat) * 2.0
	if intensity > 1.0 {
		return intensity - 1.0
	}
	return 0
}

// Grid - grid minimal que aparece con el audio
func patternGrid(x, y float64, p params.Parameters, t float64) float64 {
	gridX := x*3.0 + t*0.5
	gridY := y*3.0 - t*0.3
	lineX := gridX - math.Floor(gridX)
	lineY := gridY - math.Floor(gridY)
	if lineX > 0.9 || lineY > 0.9 {
		return p.Amplitude * 2.0
	}
	return 0
}

// Spark - chispas que explotan desde el centro
func patternSpark(x, y float64, p params.Parameters, t float64) float64 {
	angle := math.Atan2(y, x)
	rays := angle*2.0 + t*2.0
	rayVal := rays - math.Floor(rays)
	if rayVal > 0.8 {
		r := x*x + y*y
		return (1.0 / (1.0 + r)) * p.BeatDistortion * 3.0
	}
	return 0
}

// Pulse - pulso concéntrico minimal
func patternPulse(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Sqrt(x*x + y*y)
	wave := r - t*2.0
	ring := wave - math.Floor(wave)
	if ring > 0.5 {
		ring = 1.0 - ring
	}
	ring *= 2.0
	beat := p.BeatDistortion * 2.0
	return ring * ring * (1.0 + beat)
}

// Scatter - partículas dispersas
func patternScatter(x, y float64, p params.Parameters, t float64) float64 {
	cellX := math.Floor(x*5.0 + t)
	cellY := math.Floor(y*5.0 + t*0.8)
	noise := hash2(cellX, cellY)
	threshold := 0.92 - p.Amplitude*0.2
	if noise > threshold {
		return (noise - threshold) * 12.0
	}
	return 0
}

// Beam - rayos verticales que reaccionan al audio
func patternBeam(x, y float64, p params.Parameters, t float64) float64 {
	beamPos := (t * 0.3)
	beamPos = beamPos - math.Floor(beamPos)
	beamPos = (beamPos - 0.5) * 1.6
	dist := x - beamPos
	if dist < 0 {
		dist = -dist
	}
	if dist < 0.1 {
		return (0.1 - dist) * 10.0 * p.Amplitude
	}
	return 0
}

// Ripple - ondas desde centro
func patternRipple(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Sqrt(x*x + y*y)
	wave := r*3.0 - t*3.0
	ripple := wave - math.Floor(wave)
	if ripple > 0.5 {
		ripple = 1.0 - ripple
	}
	return ripple * 2.0 * p.Amplitude
}

// Strobe - efecto estroboscópico
func patternStrobe(x, y float64, p params.Parameters, t float64) float64 {
	phase := t*4.0 + p.BeatDistortion*2.0
	strobe := phase - math.Floor(phase)
	if strobe > 0.5 {
		r := x*x + y*y
		return (1.0 - r*0.3) * 2.0
	}
	return 0
}

// Particle - sistema de partículas minimal
func patternParticle(x, y float64, p params.Parameters, t float64) float64 {
	// Solo 2 partículas (mucho más rápido)
	speed := 0.4 + p.Amplitude*0.4
	best := 0.0
	for i := 0.0; i < 2.0; i++ {
		angle := (i / 2.0) * 6.28 + t
		px := math.Cos(angle) * t * speed
		py := math.Sin(angle) * t * speed
		px = math.Mod(px+2.0, 4.0) - 2.0
		py = math.Mod(py+2.0, 4.0) - 2.0
		dx := x - px
		dy := y - py
		dist := dx*dx + dy*dy
		val := 1.0 / (1.0 + dist*10.0)
		if val > best {
			best = val
		}
	}
	return best * 3.0
}

// Laser - líneas laser que cruzan la pantalla
func patternLaser(x, y float64, p params.Parameters, t float64) float64 {
	lineY := x + y*0.5 + t
	dist := lineY - math.Floor(lineY)
	if dist > 0.5 {
		dist = 1.0 - dist
	}
	if dist < 0.05 {
		beat := p.BeatDistortion * 2.0
		return (0.05 - dist) * 20.0 * (0.5 + beat)
	}
	return 0
}

// Waves - ondas minimalistas
func patternWaves(x, y float64, p params.Parameters, t float64) float64 {
	wave := x*2.0 + y*2.0 + t
	val := wave - math.Floor(wave)
	if val > 0.5 {
		val = 1.0 - val
	}
	return val * 2.0 * p.Amplitude
}

// Orbit - órbitas circulares minimal
func patternOrbit(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Sqrt(x*x + y*y)
	angle := math.Atan2(y, x)
	orbit := angle*2.0 + r*4.0 - t*2.0
	val := orbit - math.Floor(orbit)
	ringDist := r - 0.5
	if ringDist < 0 {
		ringDist = -ringDist
	}
	if ringDist < 0.3 && val > 0.7 {
		return p.Amplitude * 3.0
	}
	return 0
}

// Explosion - explosión desde el centro
func patternExplosion(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Sqrt(x*x + y*y)
	wave := r*4.0 - t*3.0
	val := wave - math.Floor(wave)
	if val > 0.5 {
		val = 1.0 - val
	}
	beat := p.BeatDistortion * 3.0
	falloff := 1.0 / (1.0 + r*r)
	return val * 2.0 * falloff * (0.3 + beat)
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
