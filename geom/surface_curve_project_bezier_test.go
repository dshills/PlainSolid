package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactComposedBezierCurvePreservesSpatialRationalTrim(t *testing.T) {
	ctx := context.Background()
	controls := [][4]*big.Rat{
		{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)},
		{big.NewRat(2, 1), big.NewRat(1, 3), big.NewRat(0, 1), big.NewRat(2, 1)},
		{big.NewRat(3, 1), big.NewRat(3, 1), big.NewRat(9, 4), big.NewRat(3, 1)},
		{big.NewRat(2, 1), big.NewRat(0, 1), big.NewRat(4, 1), big.NewRat(4, 1)},
	}
	var opts ExactCurvePlaneEventOptions
	curve, err := NewExactHomogeneousBezier(ctx, controls, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ends := [2]*big.Rat{big.NewRat(2, 7), big.NewRat(4, 5)}
	var roots [2]*ExactAlgebraicRoot
	for i, q := range ends {
		roots[i], _, err = NewExactRationalRoot(ctx, q, opts)
		if err != nil {
			t.Fatal(err)
		}
	}
	cut, err := source.RestrictAlgebraic(ctx, roots[0], roots[1], opts)
	if err != nil {
		t.Fatal(err)
	}
	result, work, err := cut.Cell.ExactBezierCurve(ctx, ends, opts)
	if err != nil || work <= 0 {
		t.Fatalf("conversion: %v (%d operations)", err, work)
	}
	if a, b := result.Domain(); a != 0 || b != 1 {
		t.Fatalf("normalized domain = [%v,%v]", a, b)
	}
	cell, err := NewExactComposedBezierCell(ctx, result, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, station := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 13), big.NewRat(2, 3), big.NewRat(1, 1)} {
		u := new(big.Rat).Add(ends[0], new(big.Rat).Mul(station, new(big.Rat).Sub(ends[1], ends[0])))
		// Independently evaluate the source homogeneous cubic by de Casteljau.
		want := make([][4]*big.Rat, len(controls))
		copy(want, controls)
		for n := len(want) - 1; n > 0; n-- {
			for i := 0; i < n; i++ {
				for d := range want[i] {
					delta := new(big.Rat).Sub(want[i+1][d], want[i][d])
					want[i][d] = delta.Add(want[i][d], delta.Mul(delta, u))
				}
			}
		}
		got, err := cell.EvaluateRational(ctx, station, opts)
		if err != nil {
			t.Fatal(err)
		}
		for d := range got.Point {
			x := new(big.Rat).Quo(want[0][d], want[0][3])
			if got.Point[d].Cmp(x) != 0 {
				t.Fatalf("at %s coordinate %d = %v, want %v", station, d, got.Point[d], x)
			}
		}
	}
	wrong := [2]*big.Rat{big.NewRat(1, 4), ends[1]}
	if c, _, e := cut.Cell.ExactBezierCurve(ctx, wrong, opts); c != nil || !errors.Is(e, ErrDomain) {
		t.Fatalf("wrong endpoint: %v %v", c, e)
	}
	if c, _, e := cut.Cell.ExactBezierCurve(ctx, ends, ExactCurvePlaneEventOptions{MaxOperations: 1}); c != nil || !errors.Is(e, ErrConvergence) {
		t.Fatalf("work limit: %v %v", c, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if c, work, e := cut.Cell.ExactBezierCurve(canceled, ends, opts); c != nil || work != 0 || !errors.Is(e, context.Canceled) {
		t.Fatalf("cancellation: %v %d %v", c, work, e)
	}
}

func TestComposedProjectBezierNonbinaryCuts(t *testing.T) {
	ctx := context.Background()
	source := composedTouchFixture(t, false)
	ends := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(5, 7)}
	var roots [2]*ExactAlgebraicRoot
	for i, q := range ends {
		roots[i], _, _ = NewExactRationalRoot(ctx, q, ExactCurvePlaneEventOptions{})
	}
	cut, e := source.RestrictAlgebraic(ctx, roots[0], roots[1], ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := cut.Cell.ProjectBezier2D(ctx, [2]int{2, 0}, ends, ExactCurvePlaneEventOptions{})
	if e != nil || r.Curve == nil || r.OriginalRange[0].Cmp(ends[0]) != 0 {
		t.Fatal(r, e)
	}
	cell, e := NewExactComposedBezierCell(ctx, r.Curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []*big.Rat{new(big.Rat), big.NewRat(1, 5), big.NewRat(2, 3), big.NewRat(1, 1)} {
		p, e := cell.EvaluateRational(ctx, s, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		u := new(big.Rat).Add(ends[0], new(big.Rat).Mul(new(big.Rat).Sub(ends[1], ends[0]), s))
		z := new(big.Rat).Sub(new(big.Rat).Mul(u, u), big.NewRat(1, 2))
		z.Mul(z, z)
		if p.Point[0].Cmp(z) != 0 || p.Point[1].Cmp(u) != 0 || p.Point[2].Sign() != 0 {
			t.Fatal(p, u, z)
		}
	}
	bad := ends
	bad[0] = big.NewRat(1, 4)
	r, e = cut.Cell.ProjectBezier2D(ctx, [2]int{2, 0}, bad, ExactCurvePlaneEventOptions{})
	if e != ErrDomain || r.Curve != nil {
		t.Fatal("wrong hint", r, e)
	}
	r, e = cut.Cell.ProjectBezier2D(ctx, [2]int{2, 0}, ends, ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || r.Curve != nil {
		t.Fatal("atomic", r, e)
	}
}
func TestComposedProjectBezierRejectAlgebraicEnd(t *testing.T) {
	ctx := context.Background()
	source := composedTouchFixture(t, false)
	hits, e := source.IntersectRawPlane(ctx, [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	end, _ := source.EndpointRoot(1)
	cut, e := source.RestrictAlgebraic(ctx, hits.Events[0].Root(), end, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	interval := hits.Events[0].Interval()
	mid := new(big.Rat).Quo(new(big.Rat).Add(interval.Lower, interval.Upper), big.NewRat(2, 1))
	r, e := cut.Cell.ProjectBezier2D(ctx, [2]int{0, 2}, [2]*big.Rat{mid, big.NewRat(1, 1)}, ExactCurvePlaneEventOptions{})
	if e != ErrDomain || r.Curve != nil {
		t.Fatal(r, e)
	}
}
