package geom

import (
	"errors"
	"testing"
)

func TestCurvePairDistanceParallelSkewContact(t *testing.T) {
	a, _ := NewBezier([]Vec3{{}, {X: 1}}, nil)
	for _, tc := range []struct {
		poles    []Vec3
		distance float64
	}{{[]Vec3{{Y: 2}, {X: 1, Y: 2}}, 4}, {[]Vec3{{.5, -1, 2}, {.5, 1, 2}}, 4}, {[]Vec3{{.5, -1, 0}, {.5, 1, 0}}, 0}} {
		b, _ := NewBezier(tc.poles, nil)
		r, e := BoundCurvePairDistance(a, b, CurvePairDistanceOptions{})
		if e != nil || !r.Complete || !r.HasWitness || r.SquaredDistanceMin > tc.distance || r.SquaredDistanceMax < tc.distance {
			t.Fatal(r, e)
		}
	}
}
func TestCurvePairDistanceRationalCurved(t *testing.T) {
	circle, _ := NewCircle(Vec3{}, Vec3{Z: 1}, 2)
	a, _ := circle.RationalBSpline()
	a, _ = a.InsertKnot(.13, 1)
	b, _ := NewBezier([]Vec3{{3, -1, 0}, {3, 1, 0}}, nil)
	r, e := BoundCurvePairDistance(a, b, CurvePairDistanceOptions{SquaredTolerance: 1e-4})
	if e != nil || !r.Complete || r.SquaredDistanceMin > 1 || r.SquaredDistanceMax < 1 {
		t.Fatal(r, e)
	}
	reverse, e := BoundCurvePairDistance(b, a, CurvePairDistanceOptions{SquaredTolerance: 1e-4})
	if e != nil || !reverse.Complete || reverse.SquaredDistanceMin > 1 || reverse.SquaredDistanceMax < 1 {
		t.Fatal(reverse, e)
	}
}
func TestCurvePairDistanceLimits(t *testing.T) {
	a, _ := NewBezier([]Vec3{{}, {1, 2, 0}, {2, 0, 0}}, nil)
	b, _ := NewBezier([]Vec3{{.3, .2, 1}, {.4, .2, 1}}, nil)
	for _, o := range []CurvePairDistanceOptions{{MaxPairs: 1}, {MaxOperations: 1}, {MaxQueuedPoles: 8}} {
		r, e := BoundCurvePairDistance(a, b, o)
		if !errors.Is(e, ErrConvergence) || r.Complete || r.SquaredDistanceMin > r.SquaredDistanceMax {
			t.Fatal(r, e)
		}
	}
}
