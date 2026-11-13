package geom

import (
	"context"
	"math/big"
	"testing"
)

func affineParameterFixture(t *testing.T, reverse, perturb bool) *ExactComposedCurveCell {
	t.Helper()
	ctx := context.Background()
	degree := 3
	powers := [][]int64{{0, 0, 1}, {0, 0, 0, 1}, nil, {1}}
	if reverse {
		degree = 4
		powers = [][]int64{{1, -1, -1, 1}, {1, -2, 0, 2, -1}, nil, {1, 1}}
	}
	net := make([][4]*big.Rat, degree+1)
	for i := range net {
		for axis, p := range powers {
			net[i][axis] = new(big.Rat)
			for j := 0; j <= i && j < len(p); j++ {
				q := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(int64(degree), int64(j)))
				net[i][axis].Add(net[i][axis], new(big.Rat).Mul(big.NewRat(p[j], 1), q))
			}
		}
		if perturb {
			net[i][2] = new(big.Rat).Mul(net[i][3], big.NewRat(1, 1000))
		}
	}
	lo, hi := 0., 1.
	if reverse {
		lo, hi = 2, 4
	}
	knots := make([]float64, 2*(degree+1))
	for i := range knots {
		knots[i] = lo
		if i > degree {
			knots[i] = hi
		}
	}
	curve, e := NewExactHomogeneousSpline(ctx, degree, net, knots, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cells, e := curve.ModelContactCells(ctx, SurfacePatchOptions{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	return cells.Cells[0]
}
func TestComposedAffineParameterWeightedReversal(t *testing.T) {
	ctx := context.Background()
	a, b := affineParameterFixture(t, false, false), affineParameterFixture(t, true, false)
	offset, scale := big.NewRat(4, 1), big.NewRat(-2, 1)
	r, e := CompareComposedCurvesAffineParameter(ctx, a, b, offset, scale, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || !r.Equivalent || !r.Reversed {
		t.Fatal(r, e)
	}
	// Neither x=t² nor y=t³ supplies an affine model projection.
	root, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-1", "0", "2"}, Lower: "1/2", Upper: "1"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	aEnd, _ := a.EndpointRoot(1)
	aa, e := a.RestrictAlgebraic(ctx, root, aEnd, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	mapped, _, e := root.AffineMap(ctx, offset, scale, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bStart, _ := b.EndpointRoot(0)
	bb, e := b.RestrictAlgebraic(ctx, bStart, mapped, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = CompareComposedCurvesAffineParameter(ctx, aa.Cell, bb.Cell, offset, scale, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || !r.Equivalent {
		t.Fatal(r, e)
	}
	wrong := affineParameterFixture(t, true, true)
	r, e = CompareComposedCurvesAffineParameter(ctx, a, wrong, offset, scale, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Equivalent {
		t.Fatal("coefficient mismatch", r, e)
	}
	r, e = CompareComposedCurvesAffineParameter(ctx, a, b, big.NewRat(3, 1), scale, ExactCurvePlaneEventOptions{})
	if e != ErrDomain || r.Proven {
		t.Fatal("domain", r, e)
	}
	r, e = CompareComposedCurvesAffineParameter(ctx, a, b, offset, scale, ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || r.Proven {
		t.Fatal("atomic", r, e)
	}
}
