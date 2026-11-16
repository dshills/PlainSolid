package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicExactNonuniformBezierCurveWindows(t *testing.T) {
	ctx := context.Background()
	original, err := NewPeriodicBSplineWithExactWeights(2,
		[]Vec3{{1, 0, 0}, {3, 2, 1}, {2, -1, 3}, {-1, 1, 2}}, []float64{1, 3, 2, 4}, -.25, 1.75)
	if err != nil {
		t.Fatal(err)
	}
	c, err := original.ExactCyclicSpline(ctx, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		c, err = c.InsertKnot(ctx, big.NewRat(-1, 8), SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	cycle := new(big.Int).Lsh(big.NewInt(1), 100)
	seam := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).SetInt(cycle), big.NewRat(7, 4)), big.NewRat(1, 4))
	start := new(big.Rat).Sub(seam, big.NewRat(1, 4))
	end := new(big.Rat).Add(seam, big.NewRat(1, 4))
	pieces, work, err := c.ExactBezierSpans(ctx, start, end, ExactBezierSpanOptions{})
	if err != nil || len(pieces) != 3 {
		t.Fatalf("want exactly three nonempty spans, got %d: %v", len(pieces), err)
	}
	if pieces[0].Start.Cmp(start) != 0 || pieces[0].End.Cmp(seam) != 0 || pieces[1].End.Cmp(new(big.Rat).Add(seam, big.NewRat(1, 8))) != 0 || pieces[2].End.Cmp(end) != 0 ||
		pieces[0].Span != 5 || pieces[1].Span != 0 || pieces[2].Span != 2 || pieces[0].Cycle.Cmp(new(big.Int).Sub(cycle, big.NewInt(1))) != 0 || pieces[2].Cycle.Cmp(cycle) != 0 {
		t.Fatal("seam, clipped endpoints, repeated knot slots or ownership changed", pieces)
	}
	for _, piece := range pieces {
		lambda := big.NewRat(7, 11)
		x := new(big.Rat).Sub(piece.End, piece.Start)
		x.Add(x.Mul(x, lambda), piece.Start)
		want, _ := periodicRefinedCurveJet(t, original, x)
		got := evaluatePeriodicBezier(piece.Control, lambda)
		for i := range want {
			if new(big.Rat).Quo(got[i], got[3]).Cmp(want[i]) != 0 {
				t.Fatal("extracted piece changed original geometry", x, got, want)
			}
		}
	}
	for _, o := range []ExactBezierSpanOptions{{MaxSpans: 2}, {SurfacePatchOptions: SurfacePatchOptions{MaxOperations: work - 1}}} {
		if got, _, err := c.ExactBezierSpans(ctx, start, end, o); got != nil || !errors.Is(err, ErrConvergence) {
			t.Fatal("limited extraction published partial spans", err)
		}
	}
}

func TestPeriodicExactNonuniformBezierTensorWindows(t *testing.T) {
	ctx := context.Background()
	data := BSplineSurfaceData{DegreeU: 2, DegreeV: 1, CountU: 4, CountV: 4, KnotsV: []float64{0, 0, .5, .5, 1, 1}}
	for u := range data.CountU {
		for v := range data.CountV {
			data.Poles = append(data.Poles, Vec3{float64(u), float64((u + v) % 3), float64(v)})
			data.Weights = append(data.Weights, float64(1+(u+v)%4))
		}
	}
	s, err := NewPeriodicBSplineSurface(data, &PeriodicSplineAxis{Period: 3}, nil)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.ExactCyclicSurface(ctx, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	knot := big.NewRat(1, 7)
	c, err = c.InsertPeriodicKnot(ctx, 0, knot, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	u := ExactParameterInterval{Lower: big.NewRat(-1, 2), Upper: big.NewRat(1, 2)}
	v := ExactParameterInterval{Lower: big.NewRat(1, 8), Upper: big.NewRat(7, 8)}
	cells, work, err := c.ExactBezierCells(ctx, u, v, PeriodicSurfaceBezierOptions{})
	if err != nil || len(cells) != 6 || cells[3].U.Domain.Upper.Cmp(knot) != 0 || cells[0].U.Cycle.Int64() != -1 ||
		cells[2].U.Cycle.Sign() != 0 || cells[0].V.Span != 0 || cells[1].V.Span != 2 {
		t.Fatal("wrong original cells", len(cells), err)
	}
	for i, cell := range cells {
		patch, err := cell.NormalizedPatch(ctx, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		a, b := big.NewRat(1, 3), big.NewRat(2, 5)
		du := new(big.Rat).Sub(cell.U.Domain.Upper, cell.U.Domain.Lower)
		dv := new(big.Rat).Sub(cell.V.Domain.Upper, cell.V.Domain.Lower)
		x := new(big.Rat).Add(cell.U.Domain.Lower, new(big.Rat).Mul(du, a))
		y := new(big.Rat).Add(cell.V.Domain.Lower, new(big.Rat).Mul(dv, b))
		want, err := s.EvaluateRational(ctx, x, y, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		got, err := patch.EvaluateRational(ctx, a, b, SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for d := range got.Point {
			if got.Point[d].Cmp(want.Point[d]) != 0 || got.Du[d].Quo(got.Du[d], du).Cmp(want.Du[d]) != 0 || got.Dv[d].Quo(got.Dv[d], dv).Cmp(want.Dv[d]) != 0 {
				t.Fatalf("cell %d differs from original surface at %s,%s", i, x, y)
			}
		}
		// V=1 at the left branch and V=0 at the right branch are
		// different original limits of the discontinuous ordinary station.
		limit, err := patch.EvaluateRational(ctx, a, big.NewRat(int64(1-i%2), 1), SurfacePatchEvaluationOptions{})
		if err != nil || limit.Point[2].Cmp(big.NewRat(int64(1+i%2), 1)) != 0 {
			t.Fatal("the original one-sided ordinary limit was averaged", i, limit, err)
		}
	}
	cells[0].U.Domain.Lower.SetInt64(-99)
	if cells[1].U.Domain.Lower.Cmp(u.Lower) != 0 {
		t.Fatal("cell ownership is aliased")
	}
	for _, o := range []PeriodicSurfaceBezierOptions{{MaxCells: 5}, {SurfacePatchOptions: SurfacePatchOptions{MaxOperations: work - 1}}} {
		if got, _, err := c.ExactBezierCells(ctx, u, v, o); got != nil || !errors.Is(err, ErrConvergence) {
			t.Fatal("limited extraction published partial cells", err)
		}
	}
}
