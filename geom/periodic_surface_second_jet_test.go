package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicSurfaceSecondJetRationalPhysicalPartials(t *testing.T) {
	ctx := context.Background()
	data := BSplineSurfaceData{DegreeU: 1, DegreeV: 1, CountU: 3, CountV: 2, KnotsV: []float64{-2, -2, 1, 1}}
	// The first span has homogeneous law (3x, 3y, 3xy, 1+x+y)
	// with physical widths 2 and 3. The factor of three makes all the
	// Euclidean pole coordinates exact in float64.
	for _, x := range []float64{0, 1, 0} {
		for _, y := range []float64{0, 1} {
			w := 1 + x + y
			data.Poles = append(data.Poles, Vec3{x * (3 / w), y * (3 / w), x * y * (3 / w)})
			data.Weights = append(data.Weights, w)
		}
	}
	uniform, err := NewPeriodicBSplineSurface(data, &PeriodicSplineAxis{Start: 2, Period: 6}, nil)
	if err != nil {
		t.Fatal(err)
	}
	exact, err := uniform.ExactCyclicSurface(ctx, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	refined, err := exact.InsertPeriodicKnot(ctx, 0, big.NewRat(7, 3), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	remote := new(big.Int).Lsh(big.NewInt(-1), 100)
	u := new(big.Rat).Add(big.NewRat(5, 2), new(big.Rat).SetInt(new(big.Int).Mul(remote, big.NewInt(6))))
	v := big.NewRat(-1, 1)
	type evaluate func(context.Context, *big.Rat, *big.Rat, SurfacePatchOptions) (RationalPeriodicSurfaceSecondJet, error)
	type first func(context.Context, *big.Rat, *big.Rat, SurfacePatchOptions) (RationalPeriodicSurfaceJet, error)
	for _, tc := range []struct {
		name   string
		second evaluate
		first  first
		span   int
	}{{"uniform", uniform.EvaluateSecondJetRational, uniform.EvaluateRational, 0},
		{"exact", exact.EvaluateSecondJetRational, exact.EvaluateRational, 0},
		{"inserted", refined.EvaluateSecondJetRational, refined.EvaluateRational, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.second(ctx, u, v, SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if got.U.Span != tc.span || got.U.Cycle.Cmp(remote) != 0 || got.V.Cycle != nil {
				t.Fatal("original span or distant cycle lost", got.U, got.V)
			}
			lower, err := tc.first(ctx, u, v, SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for i := range 3 {
				if got.Point[i].Cmp(lower.Point[i]) != 0 || got.Du[i].Cmp(lower.Du[i]) != 0 || got.Dv[i].Cmp(lower.Dv[i]) != 0 {
					t.Fatal("first jet disagrees with independent evaluation", i)
				}
				// Independently differentiating (3x,3y,3xy)/(1+x+y)
				// and applying the physical 2, 3 scale gives these values.
				want := [3][3]int64{{-1152, 288, -384}, {-312, -264, 504}, {96, -480, -120}}
				for order, d := range [3][3]*big.Rat{got.Duu, got.Duv, got.Dvv} {
					q := big.NewRat(3*want[order][i], 6859)
					if d[i].Cmp(q) != 0 {
						t.Fatalf("partial %d coordinate %d: got %s, want %s", order, i, d[i], q)
					}
				}
			}
			short := SurfacePatchOptions{MaxOperations: got.ExactOperations - 1}
			failed, err := tc.second(ctx, u, v, short)
			if !errors.Is(err, ErrConvergence) || failed.U.Cycle != nil || failed.Duv[0] != nil {
				t.Fatal("exhausted evaluation published a partial jet", err)
			}
			knot := new(big.Rat).Add(u, big.NewRat(3, 2))
			right, err := tc.second(ctx, knot, v, SurfacePatchOptions{})
			if err != nil || right.U.Span != tc.span+1 || right.U.Domain.Lower.Cmp(knot) != 0 {
				t.Fatal("interior corner did not use its right-hand span", right.U, err)
			}
			// At x=1 the next piece traverses x backward. Both the first
			// U partial and the mixed partial must carry that sign.
			if right.Du[0].Cmp(big.NewRat(-18, 49)) != 0 || right.Duv[2].Cmp(big.NewRat(-81, 686)) != 0 {
				t.Fatal("corner uses derivatives from the wrong side", right.Du, right.Duv)
			}
		})
	}
}
