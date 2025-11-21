package analyzer

import (
	"math"

	"github.com/mjibson/go-dsp/fft"
)

// Analyzer performs FFT-based spectral analysis to extract audio-reactive features.
type Analyzer struct {
	sampleRate float64

	bassPeak     float64
	midPeak      float64
	treblePeak   float64
	beatPulse    float64
	lastBass     float64
	bassHistory  []float64
	energyHist   []float64
	dropCooldown float64

	historySize int

	buffer []complex128
	window []float64
}

// Config controls Analyzer behavior.
type Config struct {
	SampleRate  float64
	HistorySize int
}

// New creates an Analyzer with sensible defaults mirroring the Rust implementation.
func New(cfg Config) *Analyzer {
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 44_100
	}
	if cfg.HistorySize <= 0 {
		cfg.HistorySize = 60
	}
	return &Analyzer{
		sampleRate:  cfg.SampleRate,
		bassHistory: make([]float64, 0, cfg.HistorySize/2),
		energyHist:  make([]float64, 0, cfg.HistorySize),
		historySize: cfg.HistorySize,
	}
}

// Analyze returns audio features for the provided mono samples and frame delta.
func (a *Analyzer) Analyze(samples []float32, deltaTime float64) Features {
	if len(samples) == 0 {
		return Features{}
	}

	size := nextPow2(min(len(samples), 2048))
	if size < 256 {
		size = 256
	}

	a.ensureWorkspace(size)

	buffer := a.buffer[:size]
	window := a.window[:size]

	sampleCount := len(samples)
	for i := 0; i < size; i++ {
		if i < sampleCount {
			buffer[i] = complex(float64(samples[i])*window[i], 0)
			continue
		}
		buffer[i] = 0
	}

	fftRes := fft.FFT(buffer)

	freqResolution := a.sampleRate / float64(size)
	bass := a.bandEnergy(fftRes, freqResolution, 20, 250)
	mid := a.bandEnergy(fftRes, freqResolution, 250, 2000)
	treble := a.bandEnergy(fftRes, freqResolution, 2000, 8000)

	a.bassPeak = envelope(a.bassPeak, bass, 0.94, 0.75)
	a.midPeak = envelope(a.midPeak, mid, 0.94, 0.78)
	a.treblePeak = envelope(a.treblePeak, treble, 0.94, 0.8)

	bassOut := dynamics(bass, a.bassPeak)
	midOut := dynamics(mid, a.midPeak)
	trebleOut := dynamics(treble, a.treblePeak)

	overall := (bassOut + midOut + trebleOut) / 3.0
	a.pushEnergy(overall)

	energyVariance := a.energyVariance()

	bassDiff := bass - a.lastBass
	beatStrength := clamp((bassDiff * 14.0), 0, 1)

	if beatStrength > 0.12 {
		a.beatPulse = 1.0
	}
	// slower decay so beat pulse lasts longer
	a.beatPulse *= 0.88
	beatStrength = math.Min(1.0, beatStrength+a.beatPulse*0.7)

	a.pushBass(bass)
	isDrop := false
	if a.dropCooldown <= 0 {
		avg := average(a.bassHistory)
		if avg > 0 && bass > avg*2.0 && bassDiff > 0.1 {
			isDrop = true
			a.dropCooldown = 1.0
		}
	} else {
		a.dropCooldown -= deltaTime
	}
	a.lastBass = bass

	varianceMultiplier := 1.0 + energyVariance*0.65

	return Features{
		Bass:         math.Min(1.0, bassOut*varianceMultiplier),
		Mid:          math.Min(1.0, midOut*varianceMultiplier),
		Treble:       math.Min(1.0, trebleOut*varianceMultiplier),
		Overall:      math.Min(1.0, overall*varianceMultiplier),
		BeatStrength: beatStrength,
		IsDrop:       isDrop,
	}
}

func (a *Analyzer) bandEnergy(buffer []complex128, resolution float64, minHz, maxHz float64) float64 {
	if minHz >= maxHz {
		return 0
	}
	lo := int(math.Floor(minHz / resolution))
	hi := int(math.Ceil(maxHz/resolution)) + 1
	if hi > len(buffer)/2 {
		hi = len(buffer) / 2
	}
	if lo >= hi {
		return 0
	}
	sum := 0.0
	for _, val := range buffer[lo:hi] {
		sum += cmag(val)
	}
	normalized := sum / float64(hi-lo)
	if normalized > 1.0 {
		return 1.0
	}
	return normalized
}

func (a *Analyzer) pushBass(value float64) {
	a.bassHistory = append(a.bassHistory, value)
	if len(a.bassHistory) > max(24, a.historySize/2) {
		copy(a.bassHistory, a.bassHistory[1:])
		a.bassHistory = a.bassHistory[:len(a.bassHistory)-1]
	}
}

func (a *Analyzer) pushEnergy(value float64) {
	a.energyHist = append(a.energyHist, value)
	if len(a.energyHist) > a.historySize {
		copy(a.energyHist, a.energyHist[1:])
		a.energyHist = a.energyHist[:len(a.energyHist)-1]
	}
}

func (a *Analyzer) energyVariance() float64 {
	if len(a.energyHist) < 10 {
		return 0
	}
	mean := average(a.energyHist)
	sumSq := 0.0
	for _, v := range a.energyHist {
		diff := v - mean
		sumSq += diff * diff
	}
	variance := sumSq / float64(len(a.energyHist))
	return math.Min(1.0, math.Sqrt(variance))
}

func hann(i, size float64) float64 {
	return 0.5 * (1.0 - math.Cos(2.0*math.Pi*i/size))
}

func (a *Analyzer) ensureWorkspace(size int) {
	if len(a.buffer) != size {
		a.buffer = make([]complex128, size)
	}
	if len(a.window) != size {
		a.window = make([]float64, size)
		sizeF := float64(size)
		for i := range a.window {
			a.window[i] = hann(float64(i), sizeF)
		}
	}
}

func cmag(c complex128) float64 {
	return math.Sqrt(real(c)*real(c) + imag(c)*imag(c))
}

func envelope(current, input, attack, release float64) float64 {
	if input > current {
		return current*attack + input*(1-attack)
	}
	return current * release
}

func dynamics(value, peak float64) float64 {
	if peak < 0.01 {
		return value
	}
	ratio := value / peak
	if ratio < 0 {
		ratio = 0
	}
	expanded := math.Pow(ratio, 0.7) * peak
	if ratio > 0.85 {
		expanded *= 1.0 + (ratio-0.85)*2.0
	}
	if expanded > 1.0 {
		return 1.0
	}
	return expanded
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func nextPow2(n int) int {
	if n <= 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
