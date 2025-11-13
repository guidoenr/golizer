package render

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/guidoenr/chroma/go-implementation/internal/analyzer"
	"github.com/guidoenr/chroma/go-implementation/internal/params"
)

type colorMode string

const (
	colorModeChromatic colorMode = "chromatic"
	colorModeFire      colorMode = "fire"
	colorModeAurora    colorMode = "aurora"
	colorModeMono      colorMode = "mono"
)

var colorModeNames = []string{
	string(colorModeChromatic),
	string(colorModeFire),
	string(colorModeAurora),
	string(colorModeMono),
}

// ColorModeNames returns the supported color modes.
func ColorModeNames() []string {
	out := make([]string, len(colorModeNames))
	copy(out, colorModeNames)
	sort.Strings(out)
	return out
}

func parseColorMode(name string) colorMode {
	switch strings.ToLower(name) {
	case "fire":
		return colorModeFire
	case "aurora", "cool":
		return colorModeAurora
	case "mono", "monochrome", "bw", "gray":
		return colorModeMono
	default:
		return colorModeChromatic
	}
}

// Renderer converts parameter state into ASCII frames.
type Renderer struct {
	width        int
	height       int
	palette      []rune
	paletteName  string
	pattern      patternFunc
	patternName  string
	colorMode    colorMode
	colorOnAudio bool
	useANSI      bool
}

// Frame contains the rendered ASCII lines and optional status text.
type Frame struct {
	Lines  []string
	Status string
}

// New creates a Renderer.
func New(width, height int, paletteName, patternName, colorModeName string, colorOnAudio bool, useANSI bool) (*Renderer, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid dimensions: width=%d height=%d", width, height)
	}

	r := &Renderer{
		width:   width,
		height:  height,
		useANSI: useANSI,
	}
	r.Configure(paletteName, patternName, colorModeName, colorOnAudio)

	return r, nil
}

// Configure updates palette, pattern and color behaviour dynamically.
func (r *Renderer) Configure(paletteName, patternName, colorModeName string, colorOnAudio bool) {
	if paletteName == "" {
		paletteName = "default"
	}
	r.palette = Palette(paletteName)
	r.paletteName = paletteName

	key := strings.ToLower(patternName)
	if key == "" {
		key = "plasma"
	}
	if pattern, ok := patternRegistry[key]; ok {
		r.pattern = pattern
		r.patternName = key
	} else {
		r.pattern = patternRegistry["plasma"]
		r.patternName = "plasma"
	}

	r.colorMode = parseColorMode(colorModeName)
	r.colorOnAudio = colorOnAudio
}

// Resize updates the framebuffer dimensions.
func (r *Renderer) Resize(width, height int) {
	if width > 0 {
		r.width = width
	}
	if height > 0 {
		r.height = height
	}
}

func (r *Renderer) PaletteName() string { return r.paletteName }
func (r *Renderer) PatternName() string { return r.patternName }
func (r *Renderer) ColorModeName() string {
	return string(r.colorMode)
}

// Render generates a frame based on parameters and features.
func (r *Renderer) Render(p params.Parameters, feat analyzer.Features, fps float64) Frame {
	if r.width <= 0 || r.height <= 0 {
		return Frame{}
	}

	activation := r.audioActivation(feat)

	lines := make([]string, r.height)
	timeFactor := p.Time
	scale := p.Scale
	if scale <= 0 {
		scale = 1
	}

	for y := 0; y < r.height; y++ {
		var builder strings.Builder
		builder.Grow(r.width * 8)
		vy := (float64(y)/float64(r.height) - 0.5) * scale
		for x := 0; x < r.width; x++ {
			vx := (float64(x)/float64(r.width) - 0.5) * scale
			char, fg := r.samplePixel(vx, vy, p, timeFactor, feat, activation)
			if r.useANSI {
				builder.WriteString(colorCode(fg))
			}
			builder.WriteRune(char)
		}
		if r.useANSI {
			builder.WriteString("\x1b[0m")
		}
		lines[y] = builder.String()
	}

	status := fmt.Sprintf(
		"%s | palette=%s pattern=%s%s | bass %.2f mid %.2f treble %.2f beat %.2f fps %.1f",
		strings.ToUpper(string(r.colorMode)),
		r.paletteName,
		r.patternName,
		func() string {
			if r.colorOnAudio {
				return " col=AUDIO"
			}
			return ""
		}(),
		feat.Bass,
		feat.Mid,
		feat.Treble,
		feat.BeatStrength,
		fps,
	)

	return Frame{
		Lines:  lines,
		Status: status,
	}
}

func (r *Renderer) samplePixel(vx, vy float64, p params.Parameters, t float64, feat analyzer.Features, activation float64) (rune, int) {
	baseX := vx
	baseY := vy

	// Beat-driven zoom and subtle rotation
	zoom := 1.0 + p.BeatZoom*0.35*math.Sin(t*2.1)
	baseX *= zoom
	baseY *= zoom

	sinRot, cosRot := math.Sincos(t * 0.2)
	rotX := baseX*cosRot - baseY*sinRot
	rotY := baseX*sinRot + baseY*cosRot

	// Swirl distortion
	radius := math.Hypot(rotX, rotY)
	swirling := p.DistortAmplitude * (0.5 + p.BeatDistortion*0.5)
	angle := math.Atan2(rotY, rotX) + swirling*math.Exp(-radius*1.8)*math.Sin(t*1.7+radius*3.2)
	radial := radius + swirling*0.15*math.Sin(t*1.3+angle*1.5)

	distortedX := radial * math.Cos(angle)
	distortedY := radial * math.Sin(angle)

	// Noise warp
	noiseScale := math.Max(0.001, p.NoiseScale*40.0)
	warp := fractalNoise((vx+p.Time*0.15)/noiseScale, (vy-p.Time*0.12)/noiseScale)
	distortedX += warp * p.NoiseStrength * 0.35
	distortedY += warp * p.NoiseStrength * 0.35

	// Base pattern + detail
	patternValue := r.pattern(distortedX, distortedY, p, t)
	detail := fractalNoise(distortedX*2+t*0.4, distortedY*2-t*0.3)
	combined := patternValue*(1-p.NoiseStrength*0.4) + detail*(p.NoiseStrength*0.4)
	combined = clampFloat(combined, -1.0, 1.0)

	amp := clampFloat(p.Amplitude, 0.0, 3.0)
	brightness := (combined*amp + 1.0) * 0.5
	brightness = clamp01(brightness)
	brightness = math.Pow(brightness, 1.0/math.Max(0.1, p.Gamma))
	brightness = math.Pow(brightness, 1.0/math.Max(0.2, p.Contrast))
	brightness = clamp01(brightness * p.Brightness)

	// Damp brightness when idle (audio reactive mode)
	if r.colorOnAudio {
		brightness = clamp01(lerp(0.04, brightness, activation))
	}

	// Apply vignette
	if p.Vignette > 0 {
		dist := math.Min(1.0, math.Hypot(vx, vy)*2.0)
		vig := clamp01(1.0 - p.Vignette*math.Pow(dist, 1.2))
		softness := clamp01(p.VignetteSoftness)
		brightness *= lerp(1.0, vig, 1.0-softness)
	}

	brightness = clamp01(brightness)

	// Glyph selection
	sharpness := math.Max(0.2, p.GlyphSharpness)
	glyphValue := math.Pow(brightness, sharpness)
	index := clampInt(int(glyphValue*float64(len(r.palette)-1)+0.5), 0, len(r.palette)-1)

	colorIndex := 15
	if r.useANSI {
		h, s, v := r.colorFromMode(combined, brightness, p, feat, activation)
		colorIndex = hsvToANSI(h, s, v)
	}

	return r.palette[index], colorIndex
}

func (r *Renderer) colorFromMode(base, brightness float64, p params.Parameters, feat analyzer.Features, activation float64) (float64, float64, float64) {
	baseNorm := clamp01((base + 1.0) * 0.5)
	shift := math.Mod(p.ColorShift/(2*math.Pi), 1.0)
	if shift < 0 {
		shift += 1.0
	}

	var h, s, v float64
	switch r.colorMode {
	case colorModeFire:
		h = clamp01(0.02 + baseNorm*0.08 + shift*0.1)
		s = clamp01(0.7 + brightness*0.25)
		v = clamp01(0.35 + brightness*0.8 + baseNorm*0.2)
	case colorModeAurora:
		h = clamp01(0.45 + baseNorm*0.25 + shift*0.3)
		s = clamp01(0.45 + p.Saturation*0.45)
		v = clamp01(0.28 + brightness*0.85 + baseNorm*0.12)
	case colorModeMono:
		h = shift
		s = 0.0
		v = clamp01(brightness)
	default:
		h = clamp01(shift + baseNorm*0.35)
		s = clamp01(0.35 + p.Saturation*0.5)
		v = clamp01(brightness*0.9 + baseNorm*0.2)
	}

	if r.colorOnAudio {
		if feat.IsDrop {
			activation = clamp01(activation + 0.2)
		}
		s = clamp01(s * activation)
		v = clamp01(0.05 + v*activation)
		if activation < 0.08 {
			s = 0
		}
	}

	return h, s, v
}

func colorCode(index int) string {
	return fmt.Sprintf("\x1b[38;5;%dm", index)
}

func hsvToANSI(h, s, v float64) int {
	r, g, b := hsvToRGB(h, s, v)
	return rgbToANSI(r, g, b)
}

func hsvToRGB(h, s, v float64) (float64, float64, float64) {
	h = clamp01(h)
	s = clamp01(s)
	v = clamp01(v)

	if s == 0 {
		return v, v, v
	}

	hv := h * 6.0
	i := math.Floor(hv)
	f := hv - i
	p := v * (1.0 - s)
	q := v * (1.0 - s*f)
	t := v * (1.0 - s*(1.0-f))

	switch int(i) % 6 {
	case 0:
		return v, t, p
	case 1:
		return q, v, p
	case 2:
		return p, v, t
	case 3:
		return p, q, v
	case 4:
		return t, p, v
	default:
		return v, p, q
	}
}

func rgbToANSI(r, g, b float64) int {
	r = clamp01(r)
	g = clamp01(g)
	b = clamp01(b)

	// Grayscale palette for low saturation/contrast
	if math.Abs(r-g) < 0.02 && math.Abs(g-b) < 0.02 {
		gray := int(clampFloat(math.Round(r*23), 0, 23))
		return 232 + gray
	}

	ri := int(clampFloat(r*5+0.5, 0, 5))
	gi := int(clampFloat(g*5+0.5, 0, 5))
	bi := int(clampFloat(b*5+0.5, 0, 5))

	return 16 + 36*ri + 6*gi + bi
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func lerp(a, b, t float64) float64 {
	return a*(1-t) + b*t
}

func (r *Renderer) audioActivation(feat analyzer.Features) float64 {
	if !r.colorOnAudio {
		return 1.0
	}
	base := feat.Overall*1.45 + feat.BeatStrength*0.6
	if feat.IsDrop {
		base += 0.3
	}
	return clamp01(base)
}
