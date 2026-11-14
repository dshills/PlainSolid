package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestAlgebraicStockProjectedSeparation(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	s := reparameterizedGraphSleeveForFlux(t)
	pair, err := s.ClipAlgebraicGraphBand(ctx, [2][3]geom.Vec3{
		{{Z: .25}, {Y: 1, Z: .25}, {X: 1, Z: .25}},
		{{Z: .75}, {X: 1, Z: .75}, {Y: 1, Z: .75}},
	}, RationalGraphShellSectionOptions{ComposeSectionCurves: true}, RationalGraphShellRegionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	opts := geom.ExactCurvePlaneEventOptions{MaxOperations: 18000000}
	model, err := pair.PrepareVerticalMaterial(ctx, RationalGraphShellSectionOptions{ComposeSectionCurves: true}, opts)
	if err != nil {
		t.Fatal(err)
	}
	at := func(x *big.Rat) AlgebraicGraphStockOccurrence {
		p, err := placement.NewRationalRigid([3][4]*big.Rat{
			{q(1, 1), q(0, 1), q(0, 1), x}, {q(0, 1), q(1, 1), q(0, 1), q(0, 1)}, {q(0, 1), q(0, 1), q(1, 1), q(0, 1)},
		}, 4096)
		if err != nil {
			t.Fatal(err)
		}
		return AlgebraicGraphStockOccurrence{pair, model, p}
	}
	origin := at(q(0, 1))
	axis := [3]*big.Rat{q(2, 1), q(0, 1), q(0, 1)}
	one := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for _, v := range []struct {
		x, gap   *big.Rat
		relation AlgebraicGraphProjectedRelation
		before   int
	}{
		{q(3, 1), q(0, 1), AlgebraicGraphProjectionTouching, 0},
		{new(big.Rat).Add(q(3, 1), one), new(big.Rat).Mul(q(2, 1), one), AlgebraicGraphProjectionSeparated, 0},
		{new(big.Rat).Sub(q(3, 1), one), nil, AlgebraicGraphProjectionOverlapping, -1},
		{q(-4, 1), q(2, 1), AlgebraicGraphProjectionSeparated, 1},
	} {
		res, err := origin.ProjectedSeparationRational(ctx, at(v.x), axis, one, opts)
		if err != nil || res.Relation != v.relation || res.Before != v.before || res.ExactOperations == 0 {
			t.Fatal("exactly distinguish a sub-binary64 positive gap, touch and overlap", v, res, err)
		}
		if v.gap == nil {
			if res.Gap.Lower != nil || res.Gap.Upper != nil {
				t.Fatal("overlap must not publish separation", res.Gap)
			}
		} else if res.Gap.Lower.Cmp(v.gap) != 0 || res.Gap.Upper.Cmp(v.gap) != 0 {
			t.Fatal("exact directional gap", res.Gap, v.gap)
		}
		if v.relation == AlgebraicGraphProjectionSeparated {
			bounds := [2]AlgebraicGraphDirectionalBounds{res.First, res.Second}
			for i, root := range []*geom.ExactAlgebraicRoot{bounds[v.before].Max.Value, bounds[v.before^1].Min.Value} {
				r, err := root.SignPolynomial(ctx, []*big.Rat{new(big.Rat).Neg(res.PlaneOffset), q(1, 1)}, opts)
				if err != nil || r.Sign != 2*i-1 {
					t.Fatal("signed separating plane residual must be strict at each true extent", res.PlaneOffset, r, err)
				}
			}
		} else if res.PlaneOffset != nil {
			t.Fatal("touching or overlapping can never publish a strict separating plane")
		}
	}
	bad, err := origin.ProjectedSeparationRational(ctx, at(q(4, 1)), axis, one,
		geom.ExactCurvePlaneEventOptions{MaxOperations: 1})
	if !errors.Is(err, geom.ErrConvergence) || bad.First.Min.Value != nil || bad.Second.Min.Value != nil {
		t.Fatal("two-owner support proof is atomic", bad, err)
	}
}
