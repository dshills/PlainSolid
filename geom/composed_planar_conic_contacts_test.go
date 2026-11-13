package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func directedCircleCell(t *testing.T, shift int64, mirror, reverse bool, z *big.Rat) *ExactComposedCurveCell {
	t.Helper()
	net := [][4]*big.Rat{
		{big.NewRat(1, 1), new(big.Rat), new(big.Rat), big.NewRat(1, 1)},
		{big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)},
		{new(big.Rat), big.NewRat(2, 1), new(big.Rat), big.NewRat(2, 1)},
	}
	for i := range net {
		if mirror {
			net[i][0].Neg(net[i][0])
		}
		net[i][0].Add(net[i][0], new(big.Rat).Mul(big.NewRat(shift, 1), net[i][3]))
		net[i][2].Mul(z, net[i][3])
	}
	if reverse {
		net[0], net[2] = net[2], net[0]
	}
	curve, e := NewExactHomogeneousBezier(context.Background(), net, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell, e := NewExactComposedBezierCell(context.Background(), curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return cell
}

func TestDirectedPlanarCircleContacts(t *testing.T) {
	ctx := context.Background()
	a := directedCircleCell(t, 0, false, false, new(big.Rat))
	for _, reverse := range []bool{false, true} {
		for _, axes := range [][2]int{{0, 1}, {1, 0}} {
			b := directedCircleCell(t, 1, true, reverse, new(big.Rat))
			r, e := IntersectPlanarComposedConics(ctx, a, b, axes, ExactCurvePlaneEventOptions{})
			want := -1
			if reverse {
				want = -want
			}
			if axes[0] == 1 {
				want = -want
			}
			if e != nil || !r.Complete || len(r.Contacts) != 1 || r.Contacts[0].CrossingSign != want || r.Contacts[0].Tangent {
				t.Fatal(r, e)
			}
			p, e := r.Contacts[0].Pair.A.Enclose(ctx, ComposedCurveEndpointOptions{Tolerance: 1e-8})
			if e != nil || p.Bounds.Min.X > .5 || p.Bounds.Max.X < .5 || p.Bounds.Min.Y > .8660254037844387 || p.Bounds.Max.Y < .8660254037844386 {
				t.Fatal(p, e)
			}
			// Independent circle equations: at crossing x=1/2, y²=3/4.
			root := r.Contacts[0].Pair.A.root
			budget := &polynomialBudget{maxOperations: 100000, maxBits: 4096}
			sign, _, e := composedPolynomialRootSign(ctx, root, rationalPolynomial{big.NewRat(-1, 1), new(big.Rat), big.NewRat(3, 1)}, budget, 1000)
			if e != nil || sign != 0 {
				t.Fatal(sign, e)
			}
		}
	}
	// Permute the exact model coordinates into XZ without changing the curves.
	bXZ := directedCircleCell(t, 1, true, false, new(big.Rat))
	aXZ := *a
	aXZ.power[1], aXZ.power[2] = a.power[2], a.power[1]
	bXZ.power[1], bXZ.power[2] = bXZ.power[2], bXZ.power[1]
	xz, e := IntersectPlanarComposedConics(ctx, &aXZ, bXZ, [2]int{0, 2}, ExactCurvePlaneEventOptions{})
	if e != nil || !xz.Complete || len(xz.Contacts) != 1 || xz.Contacts[0].CrossingSign != -1 {
		t.Fatal(xz, e)
	}
	miss, e := IntersectPlanarComposedConics(ctx, a, directedCircleCell(t, 3, true, false, new(big.Rat)), [2]int{0, 1}, ExactCurvePlaneEventOptions{})
	if e != nil || !miss.Complete || len(miss.Contacts) != 0 {
		t.Fatal(miss, e)
	}
	tangent, e := IntersectPlanarComposedConics(ctx, a, directedCircleCell(t, 2, true, false, new(big.Rat)), [2]int{0, 1}, ExactCurvePlaneEventOptions{})
	if e != nil || !tangent.Complete || len(tangent.Contacts) != 1 || !tangent.Contacts[0].Tangent || tangent.Contacts[0].CrossingSign != 0 || tangent.Contacts[0].Pair.A.EndpointMask() != 1 || tangent.Contacts[0].Pair.B.EndpointMask() != 1 {
		t.Fatal(tangent, e)
	}
	same, e := IntersectPlanarComposedConics(ctx, a, a, [2]int{0, 1}, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, ErrConvergence) || !same.CoincidentSupport || same.Complete || len(same.Contacts) != 0 {
		t.Fatal(same, e)
	}
	near := directedCircleCell(t, 1, true, false, new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
	r, e := IntersectPlanarComposedConics(ctx, a, near, [2]int{0, 1}, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, ErrDomain) || r.Complete || len(r.Contacts) != 0 {
		t.Fatal(r, e)
	}
	r, e = IntersectPlanarComposedConics(ctx, a, a, [2]int{0, 1}, ExactCurvePlaneEventOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Contacts) != 0 {
		t.Fatal(r, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	r, e = IntersectPlanarComposedConics(cancelled, a, a, [2]int{0, 1}, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal(r, e)
	}
}
