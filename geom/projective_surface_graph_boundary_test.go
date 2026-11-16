package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestProjectiveGraphContactBoundaryKeepsBothDirectedOriginalCurves(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	b := projectiveOverlapTestChart(t, false, false)
	r, err := CertifyProjectiveGraphContactBoundary(ctx, a, b, [2]int{0, 1}, SurfaceAffineCurveOptions{})
	if err != nil || !r.Complete || !r.Overlap.Coincident {
		t.Fatal("could not extract the complete proved overlap cycle", r, err)
	}
	zero, half, one := new(big.Rat), big.NewRat(1, 2), big.NewRat(1, 1)
	want := [4][3]*big.Rat{{zero, half, zero}, {one, half, half}, {one, one, one}, {zero, one, zero}}
	ratios := [4]*big.Rat{big.NewRat(13, 17), big.NewRat(7, 9), big.NewRat(17, 13), big.NewRat(9, 7)}
	knots := [4]*big.Rat{big.NewRat(13, 64), big.NewRat(7, 8), big.NewRat(51, 64), big.NewRat(1, 8)}
	for i, e := range r.Edges {
		j := (i + 1) % 4
		if e.Corners != [2]int{i, j} || e.BClockRatio.Cmp(ratios[i]) != 0 || len(e.Owners[0].Pieces) != 1 ||
			len(e.Owners[1].Pieces) != 2 || e.Owners[1].Pieces[0].End.Cmp(knots[i]) != 0 ||
			e.Owners[1].Pieces[1].Start.Cmp(knots[i]) != 0 {
			t.Fatal("original edge clocks or knot stations were lost", i, e)
		}
		for k, x := range r.Corners[i].Point {
			if x.Cmp(want[i][k]) != 0 {
				t.Fatal("wrong physical corner", i, k, x)
			}
		}
		for _, x := range [5]*big.Rat{zero, big.NewRat(1, 7), half, big.NewRat(6, 7), one} {
			y := new(big.Rat).Mul(ratios[i], x)
			d := new(big.Rat).Add(new(big.Rat).Sub(one, x), y)
			y.Quo(y, d)
			m0 := evaluateProjectiveGraphBoundaryPiece(t, e.Owners[0], x)
			m1 := evaluateProjectiveGraphBoundaryPiece(t, e.Owners[1], y)
			for axis := range 3 {
				physical := new(big.Rat).Add(want[i][axis], new(big.Rat).Mul(x, new(big.Rat).Sub(want[j][axis], want[i][axis])))
				if m0[axis].Cmp(physical) != 0 || m1[axis].Cmp(physical) != 0 {
					t.Fatal("boundary owner lost its common physical trace", i, x, y, m0, m1)
				}
			}
		}
	}
}

func TestProjectiveGraphContactBoundaryRefusesPartialProof(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	b := projectiveOverlapTestChart(t, false, false)
	for _, trial := range []struct {
		chart *SurfacePatch
		o     SurfaceAffineCurveOptions
		want  error
	}{
		{b, SurfaceAffineCurveOptions{MaxPieces: 1}, ErrConvergence},
		{projectiveOverlapTestChart(t, false, true), SurfaceAffineCurveOptions{}, ErrConvergence},
		{arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{1 + u, v, (1 + u) * v} }), SurfaceAffineCurveOptions{}, ErrDomain},
	} {
		r, err := CertifyProjectiveGraphContactBoundary(ctx, a, trial.chart, [2]int{0, 1}, trial.o)
		if !errors.Is(err, trial.want) || r.Complete || r.Overlap.Coincident || r.Corners[0].Point[0] != nil || len(r.Edges[0].Owners[0].Pieces) != 0 {
			t.Fatal("partial contact boundary must not be published", r, err)
		}
	}
}

func evaluateProjectiveGraphBoundaryPiece(t *testing.T, c SurfaceAffineCurvePieces, station *big.Rat) [3]*big.Rat {
	t.Helper()
	for _, piece := range c.Pieces {
		if station.Cmp(piece.Start) >= 0 && station.Cmp(piece.End) <= 0 {
			x := new(big.Rat).Sub(station, piece.Start)
			x.Quo(x, new(big.Rat).Sub(piece.End, piece.Start))
			budget := &polynomialBudget{maxOperations: 1_000_000, maxBits: 16_384}
			p, err := exactCurveRationalPoint(piece.Curve.patch, x, budget)
			if err != nil {
				t.Fatal(err)
			}
			return p
		}
	}
	t.Fatal("station not in the original owner partition", station)
	return [3]*big.Rat{}
}
