package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestComposedLinearExtremaExactStationaryImage(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	zero, one := q(0, 1), q(1, 1)
	// Source curve (t,t³,0) and functional t³-t/2. The unique
	// interior stationary parameter is sqrt(1/6), and its negative
	// image is -sqrt(1/54). Neither is represented as a sample.
	bezier, err := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{
		{zero, zero, zero, one}, {q(1, 3), zero, zero, one},
		{q(2, 3), zero, zero, one}, {one, one, zero, one},
	}, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewExactComposedBezierCell(ctx, bezier, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	form := [4]*big.Rat{q(-1, 2), one}
	opts := ExactCurvePlaneEventOptions{}
	r, err := c.LinearExtremaCandidates(ctx, form, opts)
	if err != nil || r.Constant || len(r.Candidates) != 3 {
		t.Fatal("two endpoints and one stationary candidate expected", r, err)
	}
	v := r.Candidates[2]
	for _, test := range []struct {
		root *ExactAlgebraicRoot
		cst  *big.Rat
		sign int
	}{{v.Parameter, q(-1, 6), 1}, {v.Value, q(-1, 54), -1}} {
		poly, err := test.root.SignPolynomial(ctx, []*big.Rat{test.cst, zero, one}, opts)
		if err != nil || poly.Sign != 0 {
			t.Fatal("the root must satisfy the independent polynomial", poly, err)
		}
		sign, err := test.root.SignPolynomial(ctx, []*big.Rat{zero, one}, opts)
		if err != nil || sign.Sign != test.sign {
			t.Fatal("must select the correct algebraic conjugate", sign, err)
		}
	}
	// Excluding the stationary point from the true cell must eliminate
	// it, even if the unrestricted model continuation still has it.
	end, _ := c.EndpointRoot(1)
	half, _, err := NewExactRationalRoot(ctx, q(1, 2), opts)
	if err != nil {
		t.Fatal(err)
	}
	trim, err := c.RestrictAlgebraic(ctx, half, end, opts)
	if err != nil {
		t.Fatal(err)
	}
	r, err = trim.Cell.LinearExtremaCandidates(ctx, form, opts)
	if err != nil || len(r.Candidates) != 2 {
		t.Fatal("stationary point outside the true source range", r, err)
	}
	r, err = c.LinearExtremaCandidates(ctx, form, ExactCurvePlaneEventOptions{MaxOperations: 1})
	if !errors.Is(err, ErrConvergence) || len(r.Candidates) != 0 {
		t.Fatal("budget exhaustion must not expose incomplete candidates", r, err)
	}
}
