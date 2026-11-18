package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"
	"time"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func checkIndependentSplineDistance(t *testing.T, s Solid, p geom.Vec3, want, tolerance float64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r, e := s.BoundSplineBoundaryDistance(ctx, p, SplineBoundaryDistanceOptions{SquaredTolerance: tolerance})
	if e != nil || !r.Complete || !r.HasWitness || r.Face < 0 || r.Face >= len(s.faces) {
		t.Fatal("query", p, r, e)
	}
	if r.SquaredMin > want+1e-14 || r.SquaredMax < want-1e-14 || r.SquaredMax-r.SquaredMin > tolerance {
		t.Fatal("invalid distance enclosure", r, want)
	}
	if !r.Point.Valid() || !uvFinite(r.UV.Min) || !uvFinite(r.UV.Max) {
		t.Fatal("invalid witness bounds", r)
	}
}
func TestSplineDistanceIndependentBoxAndLimits(t *testing.T) {
	s, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	for _, tc := range []struct {
		p geom.Vec3
		d float64
	}{
		{geom.Vec3{X: .5, Y: .5, Z: 2}, 1}, {geom.Vec3{X: .5, Y: .5, Z: .5}, .25},
		{geom.Vec3{X: 2, Y: 2, Z: 2}, 3}, {geom.Vec3{X: .5, Y: .5, Z: 1 + 0x1p-20}, 0x1p-40},
		{geom.Vec3{X: .5, Y: .5, Z: 1}, 0},
	} {
		checkIndependentSplineDistance(t, s, tc.p, tc.d, 1e-8)
	}
	m := geom.Affine{{0, -2, 0, 3}, {2, 0, 0, -1}, {0, 0, 2, .5}}
	placed, e := s.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	checkIndependentSplineDistance(t, placed, m.Apply(geom.Vec3{X: .5, Y: .5, Z: 2}), 4, 1e-8)
	for _, o := range []SplineBoundaryDistanceOptions{{MaxQueuePoles: 4}} {
		r, e := s.BoundSplineBoundaryDistance(context.Background(), geom.Vec3{X: .3125, Y: .375, Z: 2}, o)
		if e == nil || r.Complete || r.SquaredMin > 1 || r.SquaredMax < 1 {
			t.Fatal("budget lost conservative bounds", r, e)
		}
	}
	curved := testWarpedSplinePrism(t, false, false)
	for _, o := range []SplineBoundaryDistanceOptions{{MaxCells: 1, SquaredTolerance: 1e-12}, {MaxDepth: 1, SquaredTolerance: 1e-12}} {
		r, e := curved.BoundSplineBoundaryDistance(context.Background(), geom.Vec3{X: .875, Y: 1.125, Z: 2.5}, o)
		if e == nil || r.Complete || r.SquaredMin < 0 || r.SquaredMin > r.SquaredMax {
			t.Fatal("curved budget bounds", r, e)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := s.BoundSplineBoundaryDistance(ctx, geom.Vec3{}, SplineBoundaryDistanceOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete || r.SquaredMin != 0 || !math.IsInf(r.SquaredMax, 1) {
		t.Fatal(r, e)
	}
	for _, o := range []SplineBoundaryDistanceOptions{{SquaredTolerance: -1}, {MaxCells: -1}, {MaxDepth: 129}, {MaxQueuePoles: 3}} {
		if _, e := s.BoundSplineBoundaryDistance(context.Background(), geom.Vec3{}, o); !errors.Is(e, geom.ErrInvalid) {
			t.Fatal(o, e)
		}
	}
}
func TestSplineDistanceIndependentThroughHole(t *testing.T) {
	a, b, sections := multipleOpenSplineFixture(t)
	s, _, e := CombineOpenSplineShells(context.Background(), a, b, parametric.Difference, sections, SplineShellOpenOptions{})
	if e != nil {
		t.Fatal(e)
	}
	checkIndependentSplineDistance(t, s, geom.Vec3{X: .5, Y: .5, Z: .5}, 1./16, 1e-4)
	checkIndependentSplineDistance(t, s, geom.Vec3{X: .5, Y: .5, Z: 1.125}, 5./64, 1e-4)
}
func independentSplineDistanceLens(t *testing.T) Solid {
	t.Helper()
	k := []float64{0, 0, 0, 1, 1, 1}
	coeff := []float64{.25, -.25, .25}
	build := func(lower bool) *geom.BSplineSurface {
		var p []geom.Vec3
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				z := 1.
				if lower {
					z += coeff[i] + coeff[j] - 15./128
				}
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	s, _, e := SealSplineGraphPair(context.Background(), build(true), build(false), 1e-7, intersect.SplinePlaneOptions{}, geom.SurfaceInjectivityOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func TestSplineDistanceIndependentCurvedTrim(t *testing.T) {
	slowtest.Skip(t)
	s := independentSplineDistanceLens(t)
	// Above and radially outside the circular cap: the nearest boundary is
	// the actual circle, not its enclosing rectangle or its polygonal chords.
	radial := math.Sqrt(2)*.375 - math.Sqrt(15./128)
	checkIndependentSplineDistance(t, s, geom.Vec3{X: .125, Y: .125, Z: 1.25}, radial*radial+1./16, 1e-4)
}
func TestSplineDistanceIndependentOutwardRounding(t *testing.T) {
	for _, x := range []*big.Rat{big.NewRat(1, 3), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 1100)), new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 1100))} {
		lo, hi := outwardSplineDistance(x, true), outwardSplineDistance(x, false)
		if math.IsNaN(lo) || math.IsNaN(hi) || lo < 0 || lo > hi {
			t.Fatal(lo, hi)
		}
		if !math.IsInf(lo, 1) && new(big.Rat).SetFloat64(lo).Cmp(x) > 0 {
			t.Fatal("lower rounded inward")
		}
		if !math.IsInf(hi, 1) && new(big.Rat).SetFloat64(hi).Cmp(x) < 0 {
			t.Fatal("upper rounded inward", x.RatString(), lo, hi)
		}
	}
}

func TestSplineDistanceIndependentUnderflow(t *testing.T) {
	s, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	p := geom.Vec3{X: .5, Y: .5, Z: -1e-200}
	r, e := s.BoundSplineBoundaryDistance(context.Background(), p, SplineBoundaryDistanceOptions{})
	trueSquared := new(big.Rat).SetFloat64(p.Z)
	trueSquared.Mul(trueSquared, trueSquared)
	if e != nil || !r.HasWitness || !r.Complete || r.SquaredMax <= 0 || new(big.Rat).SetFloat64(r.SquaredMax).Cmp(trueSquared) < 0 {
		t.Fatal("positive distance underflowed upper bound", r, e)
	}
}

func TestSplineDistanceIndependentCurvedInterior(t *testing.T) {
	slowtest.Skip(t)
	s := independentSplineDistanceLens(t)
	// The lower paraboloid reaches its global minimum at this chart center.
	// Every other boundary point has both greater height and radial separation.
	p := geom.Vec3{X: .5, Y: .5, Z: .5}
	squared := 2401. / 16384
	checkIndependentSplineDistance(t, s, p, squared, 1e-12)
	placement := geom.Affine{{0, -2, 0, 3}, {2, 0, 0, -1}, {0, 0, 2, .5}}
	moved, e := s.Transformed(placement)
	if e != nil {
		t.Fatal(e)
	}
	checkIndependentSplineDistance(t, moved, placement.Apply(p), 4*squared, 1e-12)
}
