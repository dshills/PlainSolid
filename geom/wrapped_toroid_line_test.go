package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactToroidGeneralLineRoots(t *testing.T) {
	ctx := context.Background()
	w := locationToroid(t, true)
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	cases := []struct {
		origin, direction Vec3
		lo, hi            float64
		roots             []*big.Rat
		mult              []int
	}{
		{Vec3{}, Vec3{X: 1}, -3, 3, []*big.Rat{big.NewRat(-5, 2), big.NewRat(-3, 2), big.NewRat(3, 2), big.NewRat(5, 2)}, []int{1, 1, 1, 1}},
		{Vec3{}, Vec3{X: 3, Y: 4}, -.5, .5, []*big.Rat{big.NewRat(-1, 2), big.NewRat(-3, 10), big.NewRat(3, 10), big.NewRat(1, 2)}, []int{1, 1, 1, 1}},
		{Vec3{Y: 1.5}, Vec3{X: 1}, -3, 3, []*big.Rat{big.NewRat(-2, 1), big.NewRat(0, 1), big.NewRat(2, 1)}, []int{1, 2, 1}},
		{Vec3{X: 2, Z: .5}, Vec3{Y: 1}, -1, 1, []*big.Rat{big.NewRat(0, 1)}, []int{4}},
		{Vec3{X: 2.5}, Vec3{Y: 1}, -1, 1, []*big.Rat{big.NewRat(0, 1)}, []int{2}},
		{Vec3{X: 2}, Vec3{Z: 2}, 0, .25, []*big.Rat{big.NewRat(1, 4)}, []int{1}},
	}
	for _, tc := range cases {
		roots, e := IntersectWrappedCircularToroidLine(ctx, w, tc.origin, tc.direction, tc.lo, tc.hi, o)
		if e != nil || len(roots) != len(tc.roots) {
			t.Fatal(tc, roots, e)
		}
		for i, want := range tc.roots {
			r := roots[i]
			if r.Min.Cmp(want) > 0 || r.Max.Cmp(want) < 0 || r.Multiplicity != tc.mult[i] {
				t.Fatal("root/multiplicity", tc, i, r, want)
			}
			if new(big.Rat).Sub(r.Max, r.Min).Cmp(new(big.Rat).SetFloat64(o.ParameterTolerance)) > 0 {
				t.Fatal("wide bracket")
			}
		}
	}
	// Independent axial radical brackets must overlap the quartic result.
	axial, e := IntersectWrappedCircularToroidAxial(ctx, w, 2.25, 0, ToroidAxialOptions{AbsoluteTolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	line, e := IntersectWrappedCircularToroidLine(ctx, w, Vec3{X: 2.25}, Vec3{Z: 1}, -1, 1, o)
	if e != nil || len(line) != 2 {
		t.Fatal(e)
	}
	for i := range line {
		if line[i].Min.Cmp(axial[i].MaxZ) > 0 || line[i].Max.Cmp(axial[i].MinZ) < 0 {
			t.Fatal("axial disagreement")
		}
	}
	small := o
	small.MaxSubdivisions = 1
	roots, e := IntersectWrappedCircularToroidLine(ctx, w, Vec3{X: 2.25}, Vec3{Z: 1}, -1, 1, small)
	if !errors.Is(e, ErrConvergence) || roots != nil {
		t.Fatal("partial isolation result", roots, e)
	}
	roots, e = IntersectWrappedCircularToroidLine(ctx, w, Vec3{Z: 2}, Vec3{X: 1}, -3, 3, o)
	if e != nil || len(roots) != 0 {
		t.Fatal("empty", roots, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	roots, e = IntersectWrappedCircularToroidLine(canceled, w, Vec3{}, Vec3{X: 1}, -3, 3, o)
	if !errors.Is(e, context.Canceled) || roots != nil {
		t.Fatal(e)
	}
}
