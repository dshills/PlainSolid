package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestRationalMonotoneLineOverlapNonlinearSpeed(t *testing.T) {
	ctx := context.Background()
	net := [][4]*big.Rat{{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 2), new(big.Rat), new(big.Rat), big.NewRat(3, 2)}, {big.NewRat(1, 1), new(big.Rat), new(big.Rat), big.NewRat(2, 1)}}
	// Exact x=t/(1+t), split at t=1/2 without reparameterization.
	c, e := NewExactHomogeneousSpline(ctx, 1, net, []float64{0, 0, .5, 1, 1}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	origin := [3]*big.Rat{big.NewRat(1, 3), new(big.Rat), new(big.Rat)}
	dir := [3]*big.Rat{big.NewRat(-2, 3), new(big.Rat), new(big.Rat)}
	limits := [2]*big.Rat{big.NewRat(-1, 8), big.NewRat(1, 8)}
	r, e := c.DiscoverRationalMonotoneLineOverlaps(ctx, origin, dir, limits, ComposedLineContactsOptions{})
	if e != nil || !r.Complete || len(r.Parts) != 2 || r.DirectionSign != -1 {
		t.Fatal(r, e)
	}
	values := []*big.Rat{big.NewRat(1, 3), big.NewRat(5, 7), big.NewRat(1, 8), big.NewRat(-1, 8)}
	roots := []*ExactAlgebraicRoot{r.Parts[0].Start.Root(), r.Parts[1].End.Root(), r.Parts[0].LineStart, r.Parts[1].LineEnd}
	for i, root := range roots {
		want, _, _ := NewExactRationalRoot(ctx, values[i], ExactCurvePlaneEventOptions{})
		cmp, e := root.Compare(ctx, want, ExactCurvePlaneEventOptions{})
		if e != nil || !cmp.Proven || cmp.Order != 0 {
			t.Fatal(i, cmp, e)
		}
	}
	if _, e = c.DiscoverMonotoneLineOverlaps(ctx, origin, dir, limits, ComposedLineContactsOptions{}); e != ErrDomain {
		t.Fatal("affine contract changed", e)
	}
	r, e = c.DiscoverRationalMonotoneLineOverlaps(ctx, origin, dir, limits, ComposedLineContactsOptions{MaxEvents: 1})
	if e == nil || r.Complete || len(r.Parts) != 0 {
		t.Fatal("atomic", r, e)
	}
	// Folded rational speed must not pass by agreeing at samples or endpoints.
	net[2][0] = new(big.Rat)
	fold, e := NewExactHomogeneousBezier(ctx, net, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = fold.DiscoverRationalMonotoneLineOverlaps(ctx, origin, dir, limits, ComposedLineContactsOptions{})
	if e == nil || r.Complete || len(r.Parts) != 0 {
		t.Fatal("fold", r, e)
	}
}
