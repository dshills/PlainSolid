package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestComposedCollectionsMobiusUnmatchedKnots(t *testing.T) {
	ctx := context.Background()
	a := unmatchedModelKnots(t, mobiusCurveFixture(t, true), 0, 1, .25)
	b := unmatchedModelKnots(t, affineParameterFixture(t, false, false), 0, 1, .5)
	m := [4]*big.Rat{big.NewRat(2, 1), big.NewRat(-2, 1), big.NewRat(2, 1), big.NewRat(-1, 1)}
	r, e := CompareComposedCollectionsMobiusParameter(ctx, a, b, m, ComposedCollectionMobiusParameterOptions{})
	if e != nil || !r.Proven || !r.Equivalent || !r.Reversed || r.Pieces != 3 {
		t.Fatal(r, e)
	}
	r, e = CompareComposedCollectionsMobiusParameter(ctx, a, b, m, ComposedCollectionMobiusParameterOptions{MaxCuts: 3})
	if e == nil || r.Proven || r.Domain[0] != nil {
		t.Fatal("atomic", r, e)
	}
	wrong := m
	wrong[2] = big.NewRat(3, 1)
	r, e = CompareComposedCollectionsMobiusParameter(ctx, a, b, wrong, ComposedCollectionMobiusParameterOptions{})
	if e != ErrDomain || r.Proven {
		t.Fatal("domain", r, e)
	}
}
func TestMapMobiusRootExactAndPole(t *testing.T) {
	ctx := context.Background()
	var domain [2]*ExactAlgebraicRoot
	domain[0], _, _ = NewExactRationalRoot(ctx, new(big.Rat), ExactCurvePlaneEventOptions{})
	domain[1], _, _ = NewExactRationalRoot(ctx, big.NewRat(1, 1), ExactCurvePlaneEventOptions{})
	root, _, _ := NewExactRationalRoot(ctx, big.NewRat(1, 3), ExactCurvePlaneEventOptions{})
	m := [4]*big.Rat{new(big.Rat), big.NewRat(2, 1), big.NewRat(1, 1), big.NewRat(1, 1)}
	image, e := MapMobiusRoot(ctx, root, domain, m, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	want, _, _ := NewExactRationalRoot(ctx, big.NewRat(1, 2), ExactCurvePlaneEventOptions{})
	cmp, e := image.Compare(ctx, want, ExactCurvePlaneEventOptions{})
	if e != nil || !cmp.Proven || cmp.Order != 0 {
		t.Fatal(cmp, e)
	}
	m[2] = big.NewRat(1, 2)
	m[3] = big.NewRat(-1, 1)
	image, e = MapMobiusRoot(ctx, root, domain, m, ExactCurvePlaneEventOptions{})
	if e != ErrDomain || image != nil {
		t.Fatal("pole", image, e)
	}
}
