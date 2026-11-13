package geom

import (
	"errors"
	"math"
	"testing"
)

func TestBracketedRoots(t *testing.T) {
	for _, tt := range []struct {
		function   func(float64) float64
		a, b, want float64
	}{{func(x float64) float64 { return x*x - 2 }, 0, 2, math.Sqrt(2)}, {func(x float64) float64 { return math.Cos(x) - x }, 0, 1, .7390851332151607}, {math.Sin, 3, 4, math.Pi}, {func(x float64) float64 { return (x - 1e140) / 1e150 }, -1e150, 1e150, 1e140}} {
		result, e := SolveBracketed(func(x float64) (float64, error) {
			if x < tt.a || x > tt.b {
				t.Fatal("escaped bracket")
			}
			return tt.function(x), nil
		}, tt.a, tt.b, RootOptions{})
		tolerance := 1e-12 + math.Abs(tt.want)*1e-12
		if e != nil || math.Abs(result.Parameter-tt.want) > tolerance || math.Abs(result.Value) > 1e-12 || result.Evaluations > 130 {
			t.Fatal(result, e, tt.want)
		}
	}
}
func TestBracketedRootFailures(t *testing.T) {
	jump := func(x float64) (float64, error) {
		if x < .3 {
			return -1, nil
		}
		return 1, nil
	}
	if _, e := SolveBracketed(jump, 0, 1, RootOptions{}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	if _, e := SolveBracketed(func(x float64) (float64, error) { return x*x + 1, nil }, -1, 1, RootOptions{}); !errors.Is(e, ErrNotBracketed) {
		t.Fatal(e)
	}
	if _, e := SolveBracketed(func(x float64) (float64, error) { return x*x - 2, nil }, 0, 2, RootOptions{MaxIterations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	sentinel := errors.New("callback failed")
	if _, e := SolveBracketed(func(x float64) (float64, error) { return 0, sentinel }, 0, 1, RootOptions{}); !errors.Is(e, sentinel) {
		t.Fatal(e)
	}
	if _, e := SolveBracketed(func(x float64) (float64, error) { return math.NaN(), nil }, 0, 1, RootOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e := SolveBracketed(nil, 0, 1, RootOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
func TestBracketedEndpointAndExtremeResiduals(t *testing.T) {
	result, e := SolveBracketed(func(x float64) (float64, error) { return x, nil }, 0, 1, RootOptions{})
	if e != nil || result.Parameter != 0 || result.Evaluations != 1 {
		t.Fatal(result, e)
	}
	result, e = SolveBracketed(func(x float64) (float64, error) { return (x - .5) * 1e308, nil }, 0, 1, RootOptions{})
	if e != nil || result.Parameter != .5 || result.Value != 0 {
		t.Fatal(result, e)
	}
}

func TestBracketedCandidateStaysInBracket(t *testing.T) {
	result, err := SolveBracketed(func(x float64) (float64, error) { return (x*x - .5) * (x + 1e-15), nil }, 0, 1, RootOptions{})
	if err != nil || result.Parameter < result.Lower || result.Parameter > result.Upper || math.Abs(result.Parameter-math.Sqrt(.5)) > 2e-12 {
		t.Fatal(result, err)
	}
}
