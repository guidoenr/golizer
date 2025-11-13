package params

import (
	"testing"

	"github.com/guidoenr/chroma/go-implementation/internal/analyzer"
)

func TestApplySilenceDecayDoesNotPanic(t *testing.T) {
	p := Defaults()
	p.ApplyFeatures(analyzer.Features{}, 1.0/60.0)
}

func TestUpdateTimeAdvances(t *testing.T) {
	p := Defaults()
	p.Speed = 1.0
	p.UpdateTime(0.5)
	if p.Time <= 0 {
		t.Fatalf("expected time to advance, got %f", p.Time)
	}
}
