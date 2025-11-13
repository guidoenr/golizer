package app

import (
	"math"
	"math/rand"
	"time"

	"github.com/guidoenr/golizer/internal/analyzer"
)

type fakeGenerator struct {
	rng       *rand.Rand
	phaseBass float64
	phaseMid  float64
	phaseHigh float64
}

func newFakeGenerator() *fakeGenerator {
	return &fakeGenerator{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (f *fakeGenerator) Next(delta float64) analyzer.Features {
	f.phaseBass += delta * 0.7
	f.phaseMid += delta * 1.2
	f.phaseHigh += delta * 2.1

	bass := 0.5 + 0.5*math.Sin(f.phaseBass) + f.rng.Float64()*0.1
	mid := 0.4 + 0.4*math.Sin(f.phaseMid+0.5) + f.rng.Float64()*0.1
	treble := 0.3 + 0.3*math.Sin(f.phaseHigh+1.0) + f.rng.Float64()*0.1

	bass = clamp01(bass)
	mid = clamp01(mid)
	treble = clamp01(treble)

	beat := math.Max(0, math.Sin(f.phaseBass*2.0))
	if f.rng.Float64() < 0.02 {
		beat = 1.0
	}

	isDrop := f.rng.Float64() < 0.005

	return analyzer.Features{
		Bass:         bass,
		Mid:          mid,
		Treble:       treble,
		Overall:      (bass + mid + treble) / 3,
		BeatStrength: clamp01(beat + f.rng.Float64()*0.1),
		IsDrop:       isDrop,
	}
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
