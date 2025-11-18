package render

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/guidoenr/golizer/internal/analyzer"
	"github.com/guidoenr/golizer/internal/params"
)

type colorMode string
type qualityMode string

const (
	colorModeChromatic colorMode = "chromatic"
	colorModeFire      colorMode = "fire"
	colorModeAurora    colorMode = "aurora"
	colorModeMono      colorMode = "mono"

	qualityHigh     qualityMode = "high"
	qualityBalanced qualityMode = "balanced"
	qualityEco      qualityMode = "eco"
)

var colorModeNames = []string{
	string(colorModeChromatic),
	string(colorModeFire),
	string(colorModeAurora),
	string(colorModeMono),
}

var qualityModeNames = []string{
	string(qualityHigh),
	string(qualityBalanced),
	string(qualityEco),
}

// ColorModeNames returns the supported color modes.
func ColorModeNames() []string {
	out := make([]string, len(colorModeNames))
	copy(out, colorModeNames)
	sort.Strings(out)
	return out
}

// QualityModeNames returns the supported quality modes.
func QualityModeNames() []string {
	out := make([]string, len(qualityModeNames))
	copy(out, qualityModeNames)
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

func parseQualityMode(name string) qualityMode {
	switch strings.ToLower(name) {
	case "eco", "low", "pi":
		return qualityEco
	case "balanced", "medium", "mid":
		return qualityBalanced
	case "high", "full", "max":
		return qualityHigh
	default:
		return qualityBalanced
	}
}

// Renderer converts parameter state into ASCII frames.
type Renderer struct {
	width         int
	height        int
	palette       []rune
	paletteName   string
	pattern       patternFunc
	patternName   string
	detailMix     float64
	colorMode     colorMode
	quality       qualityMode
	colorOnAudio  bool
	useANSI       bool
	xCoords       []float64
	yCoords       []float64
	statusBuilder strings.Builder
}

// Frame contains the rendered ASCII lines and optional status text.
type Frame struct {
	Lines  []string
	Status string
}

var (
	resetANSI       = "\x1b[0m"
	precomputedANSI [256]string
)

func init() {
	for i := range precomputedANSI {
		precomputedANSI[i] = "\x1b[38;5;" + strconv.Itoa(i) + "m"
	}
}

// New creates a Renderer.
func New(width, height int, paletteName, patternName, colorModeName, qualityName string, colorOnAudio bool, useANSI bool) (*Renderer, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid dimensions: width=%d height=%d", width, height)
	}

	r := &Renderer{
		width:   width,
		height:  height,
		useANSI: useANSI,
	}
	r.SetQuality(qualityName)
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
	if entry, ok := patternRegistry[key]; ok {
		r.pattern = entry.fn
		r.patternName = key
		r.detailMix = entry.detailMix
	} else {
		def := patternRegistry["plasma"]
		r.pattern = def.fn
		r.patternName = "plasma"
		r.detailMix = def.detailMix
	}

	r.colorMode = parseColorMode(colorModeName)
	r.colorOnAudio = colorOnAudio
}

// Resize updates the framebuffer dimensions.
func (r *Renderer) Resize(width, height int) {
	changed := false
	if width > 0 {
		if r.width != width {
			r.width = width
			changed = true
		}
	}
	if height > 0 {
		if r.height != height {
			r.height = height
			changed = true
		}
	}
	if changed {
		r.xCoords = nil
		r.yCoords = nil
	}
}

func (r *Renderer) PaletteName() string { return r.paletteName }
func (r *Renderer) PatternName() string { return r.patternName }
func (r *Renderer) ColorModeName() string {
	return string(r.colorMode)
}
func (r *Renderer) QualityName() string {
	return string(r.quality)
}

// SetQuality updates renderer quality preset.
func (r *Renderer) SetQuality(name string) {
	if name == "" {
		name = string(qualityBalanced)
	}
	mode := parseQualityMode(name)
	r.quality = mode
	setNoiseProfile(r.quality)
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

	frameCtx := r.buildFrameParams(p, timeFactor)

	width := r.width
	height := r.height
	useANSI := r.useANSI

	r.ensureCoordinateCache(width, height)
	xCoords := r.xCoords
	yCoords := r.yCoords

	numWorkers := runtime.GOMAXPROCS(0)
	if numWorkers > height {
		numWorkers = height
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	var wg sync.WaitGroup
	rowJobs := make(chan int, numWorkers)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for y := range rowJobs {
				var builder strings.Builder
				builder.Grow(width * 8)
				lastColor := -1
				vy := yCoords[y] * scale
				for x := 0; x < width; x++ {
					vx := xCoords[x] * scale
					char, fg := r.samplePixel(vx, vy, p, frameCtx, feat, activation)
					if useANSI {
						if fg != lastColor {
							builder.WriteString(colorCode(fg))
							lastColor = fg
						}
					}
					builder.WriteRune(char)
				}
				if useANSI {
					builder.WriteString(resetANSI)
				}
				lines[y] = builder.String()
			}
		}()
	}

	for y := 0; y < height; y++ {
		rowJobs <- y
	}
	close(rowJobs)
	wg.Wait()

	status := r.buildStatus(feat, fps)

	return Frame{
		Lines:  lines,
		Status: status,
	}
}

func (r *Renderer) samplePixel(vx, vy float64, p params.Parameters, ctx frameParams, feat analyzer.Features, activation float64) (rune, int) {
	baseX := vx * ctx.zoom
	baseY := vy * ctx.zoom

	rotX := baseX*ctx.cosRot - baseY*ctx.sinRot
	rotY := baseX*ctx.sinRot + baseY*ctx.cosRot

	radius := math.Hypot(rotX, rotY)
	angle := math.Atan2(rotY, rotX)
	if ctx.swirlStrength != 0 {
		strength := ctx.swirlStrength
		switch ctx.quality {
		case qualityEco:
			strength *= 0.55
		case qualityBalanced:
			strength *= 0.85
		}
		atten := math.Exp(-radius * 1.6)
		angle += strength * atten * math.Sin(ctx.time*1.5+radius*2.3)
		radius += strength * 0.12 * math.Sin(ctx.time*1.15+angle*1.4)
	}

	distortedX := radius * math.Cos(angle)
	distortedY := radius * math.Sin(angle)

	if ctx.warpStrength > 0 {
		warp := fractalNoise((vx+ctx.time*0.15)/ctx.noiseScale, (vy-ctx.time*0.12)/ctx.noiseScale)
		strength := ctx.warpStrength
		switch ctx.quality {
		case qualityEco:
			strength *= 0.35
		case qualityBalanced:
			strength *= 0.7
		}
		distortedX += warp * strength
		distortedY += warp * strength
	}

	patternValue := r.pattern(distortedX, distortedY, p, ctx.time)
	combined := patternValue
	if ctx.detailWeight > 0 {
		detail := fractalNoise(distortedX*2+ctx.time*0.4, distortedY*2-ctx.time*0.3)
		combined = patternValue*(1-ctx.detailWeight) + detail*ctx.detailWeight
	}
	combined = clampFloat(combined, -1.0, 1.0)

	brightness := (combined*ctx.amplitude + 1.0) * 0.5
	brightness = clamp01(brightness)
	switch ctx.quality {
	case qualityEco:
		brightness = brightness * (0.7 + brightness*0.3)
	default:
		brightness = math.Pow(brightness, ctx.invGamma)
		brightness = math.Pow(brightness, ctx.invContrast)
	}
	brightness = clamp01(brightness * ctx.brightnessScale)

	if r.colorOnAudio {
		brightness = clamp01(lerp(0.04, brightness, activation))
	}

	if ctx.vignette > 0 {
		dist := math.Min(1.0, math.Hypot(vx, vy)*2.0)
		vig := clamp01(1.0 - ctx.vignette*math.Pow(dist, 1.2))
		brightness *= lerp(1.0, vig, 1.0-ctx.vignetteSoft)
	}

	brightness = clamp01(brightness)

	var glyphValue float64
	if ctx.quality == qualityEco {
		glyphValue = brightness
	} else {
		glyphValue = math.Pow(brightness, ctx.glyphSharpness)
	}
	index := clampInt(int(glyphValue*float64(len(r.palette)-1)+0.5), 0, len(r.palette)-1)

	colorIndex := 15
	if r.useANSI {
		h, s, v := r.colorFromMode(combined, brightness, p, feat, activation)
		colorIndex = hsvToANSI(h, s, v)
	}

	return r.palette[index], colorIndex
}

type frameParams struct {
	time            float64
	zoom            float64
	sinRot          float64
	cosRot          float64
	noiseScale      float64
	warpStrength    float64
	detailWeight    float64
	amplitude       float64
	invGamma        float64
	invContrast     float64
	brightnessScale float64
	vignette        float64
	vignetteSoft    float64
	glyphSharpness  float64
	swirlStrength   float64
	quality         qualityMode
}

func (r *Renderer) buildFrameParams(p params.Parameters, time float64) frameParams {
	zoom := 1.0 + p.BeatZoom*0.35*math.Sin(time*2.1)
	sinRot, cosRot := math.Sincos(time * 0.2)
	noiseScale := math.Max(0.001, p.NoiseScale*40.0)
	warpStrength := p.NoiseStrength * 0.35
	detailWeight := clampFloat(r.detailMix*p.NoiseStrength, 0.0, 1.0)
	amplitude := clampFloat(p.Amplitude, 0.0, 3.0)
	invGamma := 1.0 / math.Max(0.1, p.Gamma)
	invContrast := 1.0 / math.Max(0.2, p.Contrast)
	vignetteSoft := clamp01(p.VignetteSoftness)
	swirlStrength := p.DistortAmplitude * (0.5 + p.BeatDistortion*0.5)

	switch r.quality {
	case qualityEco:
		zoom = lerp(1.0, zoom, 0.6)
		detailWeight *= 0.35
		warpStrength *= 0.6
		swirlStrength *= 0.7
	case qualityBalanced:
		detailWeight *= 0.75
		warpStrength *= 0.85
		swirlStrength *= 0.9
	}

	return frameParams{
		time:            time,
		zoom:            zoom,
		sinRot:          sinRot,
		cosRot:          cosRot,
		noiseScale:      noiseScale,
		warpStrength:    warpStrength,
		detailWeight:    detailWeight,
		amplitude:       amplitude,
		invGamma:        invGamma,
		invContrast:     invContrast,
		brightnessScale: clampFloat(p.Brightness, 0.0, 3.0),
		vignette:        clampFloat(p.Vignette, 0.0, 1.0),
		vignetteSoft:    vignetteSoft,
		glyphSharpness:  math.Max(0.2, p.GlyphSharpness),
		swirlStrength:   swirlStrength,
		quality:         r.quality,
	}
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
	if index < 0 {
		index = 0
	} else if index >= len(precomputedANSI) {
		index = len(precomputedANSI) - 1
	}
	return precomputedANSI[index]
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

func (r *Renderer) ensureCoordinateCache(width, height int) {
	if len(r.xCoords) != width {
		r.xCoords = make([]float64, width)
		if width <= 1 {
			for i := range r.xCoords {
				r.xCoords[i] = 0
			}
		} else {
			scale := 1.0 / float64(width)
			for x := range r.xCoords {
				r.xCoords[x] = float64(x)*scale - 0.5
			}
		}
	}
	if len(r.yCoords) != height {
		r.yCoords = make([]float64, height)
		if height <= 1 {
			for i := range r.yCoords {
				r.yCoords[i] = 0
			}
		} else {
			scale := 1.0 / float64(height)
			for y := range r.yCoords {
				r.yCoords[y] = float64(y)*scale - 0.5
			}
		}
	}
}

func (r *Renderer) buildStatus(feat analyzer.Features, fps float64) string {
	builder := &r.statusBuilder
	builder.Reset()
	builder.Grow(128)
	builder.WriteString(colorModeLabel(r.colorMode))
	builder.WriteString(" | palette=")
	builder.WriteString(r.paletteName)
	builder.WriteString(" pattern=")
	builder.WriteString(r.patternName)
	builder.WriteString(" quality=")
	builder.WriteString(r.QualityName())
	if r.colorOnAudio {
		builder.WriteString(" col=AUDIO")
	}
	builder.WriteString(" | bass ")
	appendFloat(builder, feat.Bass, 2)
	builder.WriteString(" mid ")
	appendFloat(builder, feat.Mid, 2)
	builder.WriteString(" treble ")
	appendFloat(builder, feat.Treble, 2)
	builder.WriteString(" beat ")
	appendFloat(builder, feat.BeatStrength, 2)
	builder.WriteString(" fps ")
	appendFloat(builder, fps, 1)
	return builder.String()
}

func colorModeLabel(mode colorMode) string {
	switch mode {
	case colorModeFire:
		return "FIRE"
	case colorModeAurora:
		return "AURORA"
	case colorModeMono:
		return "MONO"
	default:
		return "CHROMATIC"
	}
}

func appendFloat(builder *strings.Builder, value float64, precision int) {
	var buf [32]byte
	b := strconv.AppendFloat(buf[:0], value, 'f', precision, 64)
	builder.Write(b)
}
