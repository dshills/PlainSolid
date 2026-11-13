package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactCurveSegmentAlgebraicClipping(t *testing.T) {
	// C(t)=(t,t²,0) crosses y=1/2 at sqrt(1/2).
	s, _ := NewBezier([]Vec3{{}, {X: .5}, {X: 1, Y: 1}}, nil)
	c := isoIntersectionFixture(t, s)
	start, end := [3]*big.Rat{new(big.Rat), big.NewRat(1, 2), new(big.Rat)}, [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 2), new(big.Rat)}
	legacy, e := IntersectExactCurveSegment(context.Background(), c, start, end, ExactCurveSegmentOptions{})
	if !errors.Is(e, ErrConvergence) || len(legacy.Pending) != 1 || !legacy.SupportingRootsComplete {
		t.Fatal(legacy, e)
	}
	r, e := IntersectExactCurveSegmentAlgebraic(context.Background(), c, start, end, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Algebraic) != 1 || len(r.Pending) != 0 || r.Algebraic[0].AtStart || r.Algebraic[0].AtEnd {
		t.Fatal(r, e)
	}
	refined, err := c.patch.InsertUKnot(.5, 1, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	refinedResult, err := IntersectExactCurveSegmentAlgebraic(context.Background(), &SurfaceIsoCurve{patch: refined}, start, end, ExactCurveSegmentOptions{})
	if err != nil || !refinedResult.Complete || len(refinedResult.Algebraic) != 1 {
		t.Fatal(refinedResult, err)
	}
	// Reverse segment orientation leaves contact classification unchanged.
	r, e = IntersectExactCurveSegmentAlgebraic(context.Background(), c, end, start, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Algebraic) != 1 {
		t.Fatal(r, e)
	}
	// A nonincident parallel segment separated by 2^-80 has no contact.
	offset := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	start[2] = offset
	end[2] = new(big.Rat).Set(offset)
	r, e = IntersectExactCurveSegmentAlgebraic(context.Background(), c, start, end, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Algebraic)+len(r.Hits) != 0 {
		t.Fatal(r, e)
	}
}
func TestExactCurveSegmentAlgebraicEndpoint(t *testing.T) {
	// C(t)=(t²,t²,0) reaches rational endpoint (1/2,1/2,0)
	// at an irrational parameter. The segment line is not the curve support.
	s, _ := NewBezier([]Vec3{{}, {}, {X: 1, Y: 1}}, nil)
	c := isoIntersectionFixture(t, s)
	start, end := [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), new(big.Rat)}, [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 1), new(big.Rat)}
	r, e := IntersectExactCurveSegmentAlgebraic(context.Background(), c, start, end, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Algebraic) != 1 || !r.Algebraic[0].AtStart || r.Algebraic[0].AtEnd {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := IntersectExactCurveSegmentAlgebraic(ctx, c, start, end, ExactCurveSegmentOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	r, e = IntersectExactCurveSegmentAlgebraic(context.Background(), c, start, end, ExactCurveSegmentOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
}
