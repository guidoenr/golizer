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
	"flash":     {patternFlash, 0.0},
	"spark":     {patternSpark, 0.1},
	"scatter":   {patternScatter, 0.0},
	"beam":      {patternBeam, 0.0},
	"ripple":    {patternRipple, 0.1},
	"laser":     {patternLaser, 0.0},
	"orbit":     {patternOrbit, 0.0},
	"explosion": {patternExplosion, 0.1},
	"rings":     {patternRings, 0.0},
	"zigzag":    {patternZigzag, 0.0},
	"cross":     {patternCross, 0.0},
	"spiral":    {patternSpiral, 0.1},
	"star":      {patternStar, 0.0},
	"tunnel":    {patternTunnel, 0.1},
	"neurons":   {patternNeurons, 0.0},
	"fractal":   {patternFractal, 0.1},
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

// intense flashes from center on beat (sparse - only the bright center)
func patternFlash(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Sqrt(x*x + y*y)
	if r > 0.3 {
		return -1.0 // black
	}
	flash := (0.3 - r) * 3.0
	beat := p.BeatDistortion * 2.0
	intensity := flash + beat
	if intensity > 0.8 {
		return intensity
	}
	return -1.0
}

// sparks exploding from center (sparse - only the rays)
func patternSpark(x, y float64, p params.Parameters, t float64) float64 {
	angle := math.Atan2(y, x)
	rays := angle*2.5 + t*2.0
	rayVal := rays - math.Floor(rays)
	if rayVal < 0.15 || rayVal > 0.85 {
		r := math.Sqrt(x*x + y*y)
		if r < 1.2 {
			return p.BeatDistortion * 3.0 * (1.2 - r)
		}
	}
	return -1.0
}

// scattered particles (sparse - only dots)
func patternScatter(x, y float64, p params.Parameters, t float64) float64 {
	cellX := math.Floor(x*5.0 + t)
	cellY := math.Floor(y*5.0 + t*0.8)
	noise := hash2(cellX, cellY)
	threshold := 0.95 - p.Amplitude*0.1
	if noise > threshold {
		return (noise - threshold) * 20.0
	}
	return -1.0
}

// vertical beams (sparse - only the beam lines)
func patternBeam(x, y float64, p params.Parameters, t float64) float64 {
	beamPos := (t * 0.3)
	beamPos = beamPos - math.Floor(beamPos)
	beamPos = (beamPos - 0.5) * 1.6
	dist := x - beamPos
	if dist < 0 {
		dist = -dist
	}
	if dist < 0.08 {
		return (0.08 - dist) * 12.0 * p.Amplitude
	}
	return -1.0
}

// ripples from center (sparse - only the ring edges)
func patternRipple(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Sqrt(x*x + y*y)
	wave := r*3.0 - t*3.0
	ripple := wave - math.Floor(wave)
	if ripple < 0.1 || ripple > 0.9 {
		dist := math.Min(ripple, 1.0-ripple)
		return dist * 20.0 * p.Amplitude
	}
	return -1.0
}

// NEW: tunnel perspective effect (sparse - only the tunnel edges)
func patternTunnel(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Sqrt(x*x + y*y)
	if r < 0.1 {
		return -1.0
	}
	angle := math.Atan2(y, x)
	depth := 1.0/r - t*2.0
	tunnel := depth - math.Floor(depth)
	
	// draw tunnel rings
	if tunnel < 0.1 {
		angleSnap := math.Floor(angle * 8.0 / (2.0 * math.Pi))
		if math.Mod(angleSnap, 2.0) < 1.0 {
			return tunnel * 10.0 * (0.5 + p.BeatDistortion)
		}
	}
	return -1.0
}

// NEW: neural network connections (sparse - dots and connecting lines)
func patternNeurons(x, y float64, p params.Parameters, t float64) float64 {
	// create nodes
	nodes := []struct{ nx, ny float64 }{
		{math.Sin(t * 0.3), math.Cos(t * 0.4)},
		{math.Sin(t*0.5 + 2.0), math.Cos(t*0.3 - 1.0)},
		{math.Sin(t*0.4 - 1.5), math.Cos(t*0.6 + 0.5)},
	}
	
	// check if near any node
	for _, node := range nodes {
		dist := math.Sqrt((x-node.nx)*(x-node.nx) + (y-node.ny)*(y-node.ny))
		if dist < 0.12 {
			return (0.12 - dist) * 8.0 * p.Amplitude
		}
	}
	
	// check if on connection line
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			n1, n2 := nodes[i], nodes[j]
			// distance from point to line segment
			dx := n2.nx - n1.nx
			dy := n2.ny - n1.ny
			t_line := ((x-n1.nx)*dx + (y-n1.ny)*dy) / (dx*dx + dy*dy)
			if t_line >= 0 && t_line <= 1 {
				px := n1.nx + t_line*dx
				py := n1.ny + t_line*dy
				dist := math.Sqrt((x-px)*(x-px) + (y-py)*(y-py))
				if dist < 0.03 {
					return (0.03 - dist) * 15.0 * p.BeatDistortion * 2.0
				}
			}
		}
	}
	return -1.0
}

// NEW: fractal branches (sparse - only the fractal edges)
func patternFractal(x, y float64, p params.Parameters, t float64) float64 {
	angle := math.Atan2(y, x)
	r := math.Sqrt(x*x + y*y)
	
	// create fractal branches
	branches := 5.0
	branchAngle := math.Mod(angle*branches + t, 2.0*math.Pi)
	if branchAngle > math.Pi {
		branchAngle = 2.0*math.Pi - branchAngle
	}
	
	// fractal scaling
	scale := math.Sin(r*4.0 - t*2.0)
	if branchAngle < 0.2 && scale > 0.5 && r < 1.2 {
		return (0.2 - branchAngle) * 15.0 * (scale - 0.5) * (0.5 + p.Amplitude)
	}
	return -1.0
}

// laser lines crossing (sparse - only the laser lines)
func patternLaser(x, y float64, p params.Parameters, t float64) float64 {
	lineY := x + y*0.5 + t
	dist := lineY - math.Floor(lineY)
	if dist > 0.5 {
		dist = 1.0 - dist
	}
	if dist < 0.04 {
		beat := p.BeatDistortion * 2.0
		return (0.04 - dist) * 25.0 * (0.5 + beat)
	}
	return -1.0
}

// circular orbits (sparse - only the orbit paths)
func patternOrbit(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Sqrt(x*x + y*y)
	angle := math.Atan2(y, x)
	orbit := angle*2.0 + r*4.0 - t*2.0
	val := orbit - math.Floor(orbit)
	ringDist := r - 0.5
	if ringDist < 0 {
		ringDist = -ringDist
	}
	if ringDist < 0.15 && val > 0.85 {
		return p.Amplitude * 5.0
	}
	return -1.0
}

// explosion from center (sparse - only the expanding ring)
func patternExplosion(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Sqrt(x*x + y*y)
	wave := r*4.0 - t*3.0
	val := wave - math.Floor(wave)
	if val < 0.15 || val > 0.85 {
		beat := p.BeatDistortion * 3.0
		edgeDist := math.Min(val, 1.0-val)
		return edgeDist * 20.0 * (0.3 + beat)
	}
	return -1.0
}

// NEW: concentric rings pulsing (sparse)
func patternRings(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Sqrt(x*x + y*y)
	rings := math.Sin(r*8.0 - t*3.0)
	if rings > 0.7 {
		return (rings - 0.7) * 10.0 * p.Amplitude
	}
	return -1.0
}

// NEW: zigzag lightning effect (sparse)
func patternZigzag(x, y float64, p params.Parameters, t float64) float64 {
	zigX := math.Sin(y*5.0 + t*2.0) * 0.3
	dist := math.Abs(x - zigX)
	if dist < 0.06 {
		return (0.06 - dist) * 16.0 * (0.5 + p.BeatDistortion*2.0)
	}
	return -1.0
}

// NEW: cross pattern (sparse - only the cross lines)
func patternCross(x, y float64, p params.Parameters, t float64) float64 {
	angle := math.Atan2(y, x) + t
	angle = angle - math.Floor(angle/(math.Pi/2))*(math.Pi/2)
	if math.Abs(angle) < 0.1 || math.Abs(angle-math.Pi/2) < 0.1 {
		r := math.Sqrt(x*x + y*y)
		if r < 1.0 {
			return (1.0 - r) * p.Amplitude * 3.0
		}
	}
	return -1.0
}

// NEW: spiral arms (sparse)
func patternSpiral(x, y float64, p params.Parameters, t float64) float64 {
	r := math.Sqrt(x*x + y*y)
	angle := math.Atan2(y, x)
	spiral := angle*3.0 - r*8.0 + t*3.0
	val := spiral - math.Floor(spiral)
	if val < 0.12 {
		return val * 25.0 * p.Amplitude
	}
	return -1.0
}

// NEW: star burst (sparse - only the star rays)
func patternStar(x, y float64, p params.Parameters, t float64) float64 {
	angle := math.Atan2(y, x) + t
	points := 8.0
	starAngle := math.Mod(angle*points, 2.0*math.Pi)
	if starAngle > math.Pi {
		starAngle = 2.0*math.Pi - starAngle
	}
	if starAngle < 0.3 {
		r := math.Sqrt(x*x + y*y)
		if r < 1.2 && r > 0.2 {
			return (0.3 - starAngle) * 10.0 * (0.5 + p.BeatDistortion*2.0)
		}
	}
	return -1.0
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
