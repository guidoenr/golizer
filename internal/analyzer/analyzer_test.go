package analyzer

import (
	"math"
	"testing"
)

func TestAverage(t *testing.T) {
	vals := []float64{0.2, 0.4, 0.6, 0.8}
	want := 0.5
	if got := average(vals); math.Abs(got-want) > 1e-6 {
		t.Fatalf("average=%f want=%f", got, want)
	}
}

func TestNextPow2(t *testing.T) {
	cases := map[int]int{
		0:   1,
		1:   1,
		2:   2,
		3:   4,
		5:   8,
		16:  16,
		31:  32,
		257: 512,
	}
	for input, want := range cases {
		if got := nextPow2(input); got != want {
			t.Fatalf("nextPow2(%d)=%d want=%d", input, got, want)
		}
	}
}

func TestDynamicsWithLowPeakReturnsValue(t *testing.T) {
	if got := dynamics(0.5, 0.0); got != 0.5 {
		t.Fatalf("dynamics for zero peak: got=%f want=0.5", got)
	}
}

func TestClamp(t *testing.T) {
	if clamp(2, 0, 1) != 1 {
		t.Fatalf("expected clamp high to be 1")
	}
	if clamp(-1, 0, 1) != 0 {
		t.Fatalf("expected clamp low to be 0")
	}
	if clamp(0.5, 0, 1) != 0.5 {
		t.Fatalf("expected clamp middle to be unchanged")
	}
}
