package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicSurfaceRationalJetAtSeam(t *testing.T) {
	surface := periodicDistanceSurface(t)
	ctx := context.Background()
	cycle := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 100))
	seam := new(big.Rat).SetInt(cycle)
	v := big.NewRat(1, 4)
	for _, tc := range []struct {
		offset *big.Rat
		cycle  *big.Int
		span   int
		x, y   *big.Rat
		ux, uy int64
	}{
		{big.NewRat(-1, 21), new(big.Int).Sub(cycle, big.NewInt(1)), 2, big.NewRat(0, 1), big.NewRat(2, 7), 0, -6},
		{big.NewRat(0, 1), cycle, 0, big.NewRat(0, 1), big.NewRat(0, 1), 6, 0},
		{big.NewRat(1, 21), cycle, 0, big.NewRat(2, 7), big.NewRat(0, 1), 6, 0},
	} {
		u := new(big.Rat).Add(seam, tc.offset)
		jet, err := surface.EvaluateRational(ctx, u, v, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if jet.U.Cycle.Cmp(tc.cycle) != 0 || jet.U.Span != tc.span || jet.V.Cycle != nil || jet.V.Span != 1 {
			t.Fatalf("offset %v owns %+v and %+v", tc.offset, jet.U, jet.V)
		}
		if jet.Point[0].Cmp(tc.x) != 0 || jet.Point[1].Cmp(tc.y) != 0 || jet.Point[2].Cmp(big.NewRat(3, 1)) != 0 {
			t.Fatalf("offset %v point %v", tc.offset, jet.Point)
		}
		if jet.Du[0].Cmp(big.NewRat(tc.ux, 1)) != 0 || jet.Du[1].Cmp(big.NewRat(tc.uy, 1)) != 0 || jet.Du[2].Sign() != 0 || jet.Dv[0].Sign() != 0 || jet.Dv[1].Sign() != 0 || jet.Dv[2].Cmp(big.NewRat(4, 3)) != 0 {
			t.Fatalf("offset %v derivatives %v and %v", tc.offset, jet.Du, jet.Dv)
		}
		assertExactPointInBox(t, jet.Point, jet.PointBounds)
		assertExactPointInBox(t, jet.Du, jet.DuBounds)
		assertExactPointInBox(t, jet.Dv, jet.DvBounds)
		if output, err := surface.EvaluateRational(ctx, u, v, SurfacePatchOptions{MaxOperations: jet.ExactOperations - 1}); !errors.Is(err, ErrConvergence) || output.Point[0] != nil || output.U.Cycle != nil {
			t.Fatalf("short proof published geometry: %+v, error %v", output, err)
		}
	}
	last, err := surface.EvaluateRational(ctx, seam, big.NewRat(1, 1), SurfacePatchOptions{})
	if err != nil || last.Point[2].Cmp(big.NewRat(4, 1)) != 0 || last.V.Span != 1 {
		t.Fatalf("ordinary upper endpoint = %+v, error %v", last, err)
	}
	if _, err := surface.EvaluateRational(ctx, seam, big.NewRat(2, 1), SurfacePatchOptions{}); !errors.Is(err, ErrDomain) {
		t.Fatalf("ordinary axis validation: %v", err)
	}
}

func TestPeriodicSurfaceRationalJetAtDiscontinuousOrdinaryKnot(t *testing.T) {
	data := BSplineSurfaceData{
		DegreeU: 1, DegreeV: 1, CountU: 3, CountV: 4,
		KnotsV: []float64{0, 0, .5, .5, 1, 1},
		Poles:  []Vec3{{0, 0, 0}, {0, 0, 1}, {0, 0, 2}, {0, 0, 3}, {1, 0, 0}, {1, 0, 1}, {1, 0, 2}, {1, 0, 3}, {0, 1, 0}, {0, 1, 1}, {0, 1, 2}, {0, 1, 3}},
	}
	surface, err := NewPeriodicBSplineSurface(data, &PeriodicSplineAxis{Period: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, v := big.NewRat(1, 6), big.NewRat(1, 2)
	jet, err := surface.EvaluateRational(context.Background(), u, v, SurfacePatchOptions{})
	if err != nil || jet.V.Span != 2 || jet.Point[2].Cmp(big.NewRat(2, 1)) != 0 || jet.Dv[2].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatalf("full multiplicity chooses wrong side: %+v, %v", jet, err)
	}
}
