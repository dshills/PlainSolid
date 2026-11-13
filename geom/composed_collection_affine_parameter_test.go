package geom

import (
	"context"
	"math/big"
	"testing"
)

func unmatchedModelKnots(t *testing.T, cell *ExactComposedCurveCell, lo, hi, knot float64) SurfaceCurveCompositionCells {
	t.Helper()
	ctx := context.Background()
	budget := &polynomialBudget{maxOperations: 1000000, maxBits: 16384}
	degree := 4
	net := make([][4]*big.Rat, 5)
	for axis := 0; axis < 4; axis++ {
		p, e := composeProjectionPolynomial(cell.power[axis], rationalPolynomial{new(big.Rat).SetFloat64(lo), new(big.Rat).SetFloat64(hi - lo)}, budget)
		if e != nil {
			t.Fatal(e)
		}
		for i := range net {
			net[i][axis] = new(big.Rat)
			for j := 0; j <= i && j < len(p); j++ {
				q := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(4, int64(j)))
				net[i][axis].Add(net[i][axis], new(big.Rat).Mul(p[j], q))
			}
		}
	}
	knots := make([]float64, 10)
	for i := range knots {
		knots[i] = lo
		if i >= 5 {
			knots[i] = hi
		}
	}
	curve, e := NewExactHomogeneousSpline(ctx, degree, net, knots, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	left, right, e := curve.patch.split(knot, true, budget)
	if e != nil {
		t.Fatal(e)
	}
	controls := [][4]*big.Rat{}
	for _, p := range []*SurfacePatch{left, right} {
		start := 0
		if p == right {
			start = 1
		}
		for i := start; i < p.countU; i++ {
			controls = append(controls, [4]*big.Rat(p.net[i*p.countV]))
		}
	}
	knots = nil
	for i := 0; i < 5; i++ {
		knots = append(knots, lo)
	}
	for i := 0; i < 4; i++ {
		knots = append(knots, knot)
	}
	for i := 0; i < 5; i++ {
		knots = append(knots, hi)
	}
	refined, e := NewExactHomogeneousSpline(ctx, degree, controls, knots, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cells, e := refined.ModelContactCells(ctx, SurfacePatchOptions{}, 4)
	if e != nil {
		t.Fatal(e)
	}
	return cells
}
func TestComposedCollectionsAffineDifferentKnots(t *testing.T) {
	ctx := context.Background()
	a := unmatchedModelKnots(t, affineParameterFixture(t, false, false), 0, 1, .25)
	b := unmatchedModelKnots(t, affineParameterFixture(t, true, false), 2, 4, 3)
	r, e := CompareComposedCollectionsAffineParameter(ctx, a, b, big.NewRat(4, 1), big.NewRat(-2, 1), ComposedCollectionAffineParameterOptions{})
	if e != nil || !r.Proven || !r.Equivalent || !r.Reversed || r.Pieces != 3 {
		t.Fatal(r, e)
	}
	bad := unmatchedModelKnots(t, affineParameterFixture(t, true, true), 2, 4, 3)
	r, e = CompareComposedCollectionsAffineParameter(ctx, a, bad, big.NewRat(4, 1), big.NewRat(-2, 1), ComposedCollectionAffineParameterOptions{})
	if e != nil || !r.Proven || r.Equivalent {
		t.Fatal(r, e)
	}
	r, e = CompareComposedCollectionsAffineParameter(ctx, a, b, big.NewRat(4, 1), big.NewRat(-2, 1), ComposedCollectionAffineParameterOptions{MaxCuts: 3})
	if e == nil || r.Proven || r.Domain[0] != nil {
		t.Fatal("atomic cut cap", r, e)
	}
	// A public Complete flag cannot make an omitted endpoint range equivalent.
	b.Cells = b.Cells[:1]
	r, e = CompareComposedCollectionsAffineParameter(ctx, a, b, big.NewRat(4, 1), big.NewRat(-2, 1), ComposedCollectionAffineParameterOptions{})
	if e != ErrDomain || r.Proven {
		t.Fatal(r, e)
	}
}

func TestComposedCollectionsAffineAlgebraicCutUnion(t *testing.T) {
	ctx := context.Background()
	cell := affineParameterFixture(t, false, false)
	root, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-1", "0", "2"}, Lower: "1/2", Upper: "1"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	lo, _ := cell.EndpointRoot(0)
	hi, _ := cell.EndpointRoot(1)
	left, e := cell.RestrictAlgebraic(ctx, lo, root, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	right, e := cell.RestrictAlgebraic(ctx, root, hi, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a := SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{left.Cell, right.Cell}}
	b := unmatchedModelKnots(t, affineParameterFixture(t, true, false), 2, 4, 3)
	r, e := CompareComposedCollectionsAffineParameter(ctx, a, b, big.NewRat(4, 1), big.NewRat(-2, 1), ComposedCollectionAffineParameterOptions{})
	if e != nil || !r.Proven || !r.Equivalent || r.Pieces != 3 {
		t.Fatal(r, e)
	}
}
