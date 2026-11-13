package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestMonotoneLineOverlapCellsAndCaps(t *testing.T) {
	ctx := context.Background()
	net := [][4]*big.Rat{{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(3, 1), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}}
	c, e := NewExactHomogeneousSpline(ctx, 1, net, []float64{0, 0, 1, 3, 3}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	origin := [3]*big.Rat{big.NewRat(1, 3), new(big.Rat), new(big.Rat)}
	dir := [3]*big.Rat{big.NewRat(-2, 3), new(big.Rat), new(big.Rat)}
	r, e := c.DiscoverMonotoneLineOverlaps(ctx, origin, dir, [2]*big.Rat{big.NewRat(-2, 1), new(big.Rat)}, ComposedLineContactsOptions{})
	if e != nil || !r.Complete || len(r.Parts) != 2 || r.DirectionSign != -1 || r.Parts[0].Cell != 0 || r.Parts[1].Cell != 1 {
		t.Fatal(r, e)
	}
	q, e := r.Parts[0].End.CompareParameter(ctx, r.Parts[1].Start, ExactCurvePlaneEventOptions{})
	if e != nil || !q.Proven || q.Order != 0 {
		t.Fatal(q, e)
	}
	for i, root := range []*ExactAlgebraicRoot{r.Parts[0].LineStart, r.Parts[1].LineEnd} {
		value := big.NewRat(int64(-2*i), 1)
		want, _, _ := NewExactRationalRoot(ctx, value, ExactCurvePlaneEventOptions{})
		cmp, e := root.Compare(ctx, want, ExactCurvePlaneEventOptions{})
		if e != nil || !cmp.Proven || cmp.Order != 0 {
			t.Fatal(cmp, e)
		}
	}

	// Closed zero-width cap at a knot retains both incident source occurrences.
	r, e = c.DiscoverMonotoneLineOverlaps(ctx, origin, dir, [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(-1, 1)}, ComposedLineContactsOptions{})
	if e != nil || !r.Complete || len(r.Parts) != 2 || !r.Parts[0].Point || !r.Parts[1].Point {
		t.Fatal(r, e)
	}
	r, e = c.DiscoverMonotoneLineOverlaps(ctx, origin, dir, [2]*big.Rat{big.NewRat(-2, 1), new(big.Rat)}, ComposedLineContactsOptions{MaxEvents: 1})
	if e == nil || r.Complete || len(r.Parts) != 0 {
		t.Fatal("atomic", r, e)
	}
	// Folded support is rejected even though all points remain on the line.
	net[2][0] = new(big.Rat)
	fold, e := NewExactHomogeneousSpline(ctx, 1, net, []float64{0, 0, 1, 3, 3}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = fold.DiscoverMonotoneLineOverlaps(ctx, origin, dir, [2]*big.Rat{big.NewRat(-2, 1), new(big.Rat)}, ComposedLineContactsOptions{})
	if e != ErrDomain || r.Complete {
		t.Fatal(r, e)
	}
}
