package geom

import (
	"context"
	"math/big"
	"testing"
)

func graphGridFixture(t *testing.T, roots [2][]*big.Rat) (*SurfacePatch, *SurfacePatch) {
	a, b := graphContactFixture(t, false, false)
	var powers [2]rationalPolynomial
	budget := &polynomialBudget{maxOperations: 100000, maxBits: 1024}
	for axis := 0; axis < 2; axis++ {
		powers[axis] = rationalPolynomial{big.NewRat(1, 1)}
		for _, root := range roots[axis] {
			powers[axis], _ = polynomialProduct(powers[axis], rationalPolynomial{new(big.Rat).Neg(root), big.NewRat(1, 1)}, budget)
		}
	}
	coefficient := func(axis, i, n int) *big.Rat {
		r := new(big.Rat)
		for k, c := range powers[axis] {
			if k > i {
				break
			}
			f := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(int64(n), int64(k)))
			r.Add(r, new(big.Rat).Mul(c, f))
		}
		return r
	}
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			delta := new(big.Rat).Mul(coefficient(0, i, a.degreeU), coefficient(1, j, a.degreeV))
			a.net[i*a.countV+j][2].Add(b.net[i*a.countV+j][2], delta)
		}
	}
	a, _ = a.InsertUKnot(.5, 1, SurfacePatchOptions{})
	b, _ = b.InsertVKnot(.25, 1, SurfacePatchOptions{})
	return a, b
}
func TestGraphContactGridExactCompleteness(t *testing.T) {
	for _, roots := range [][2][]*big.Rat{{{big.NewRat(1, 3), big.NewRat(2, 3)}, {big.NewRat(2, 5)}}, {{big.NewRat(1, 3), big.NewRat(2, 3)}, {big.NewRat(1, 4), big.NewRat(3, 4)}}, {{}, {big.NewRat(1, 3)}}, {{}, {}}} {
		a, b := graphGridFixture(t, roots)
		for _, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			r, e := DiscoverSplineGraphContactGrid(context.Background(), pair[0], pair[1], GraphCrossingLinesOptions{})
			if e != nil || !r.Proven || len(r.Cells) != (len(roots[0])+1)*(len(roots[1])+1) || len(r.Vertices) != (len(roots[0])+2)*(len(roots[1])+2) {
				t.Fatal(r, e)
			}
			for axis := 0; axis < 2; axis++ {
				if len(r.Lines[axis]) != len(roots[axis]) {
					t.Fatal(r)
				}
				for i, x := range r.Lines[axis] {
					if x.Cmp(roots[axis][i]) != 0 {
						t.Fatal(x)
					}
				}
			}
			for _, cell := range r.Cells {
				uv := [2]*big.Rat{}
				for axis := 0; axis < 2; axis++ {
					uv[axis] = new(big.Rat).Quo(new(big.Rat).Add(cell.Lower[axis], cell.Upper[axis]), big.NewRat(2, 1))
				}
				x, e := pair[0].EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				y, e := pair[1].EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if new(big.Rat).Sub(x.Point[2], y.Point[2]).Sign() != cell.Side {
					t.Fatal("independent rectangle sign")
				}
			}
		}
	}
}
func TestGraphContactGridRejectsRepeatedPerturbedAndLimits(t *testing.T) {
	a, b := graphGridFixture(t, [2][]*big.Rat{{big.NewRat(1, 3), big.NewRat(1, 3)}, {big.NewRat(2, 5)}})
	r, e := DiscoverSplineGraphContactGrid(context.Background(), a, b, GraphCrossingLinesOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	a, b = graphGridFixture(t, [2][]*big.Rat{{big.NewRat(1, 3), big.NewRat(2, 3)}, {big.NewRat(2, 5)}})
	for _, o := range []GraphCrossingLinesOptions{{MaxCandidates: 1}, {GraphPointContactOptions: GraphPointContactOptions{MaxCells: 4}}, {GraphPointContactOptions: GraphPointContactOptions{MaxOperations: 1}}} {
		r, e = DiscoverSplineGraphContactGrid(context.Background(), a, b, o)
		if e == nil || r.Proven {
			t.Fatal(r, e)
		}
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for i := range a.net {
		a.net[i][2].Add(a.net[i][2], tiny)
	}
	r, e = DiscoverSplineGraphContactGrid(context.Background(), a, b, GraphCrossingLinesOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
}
