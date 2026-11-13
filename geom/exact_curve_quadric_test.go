package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func circleQuadric() ExactQuadric {
	return ExactQuadric{0: big.NewRat(1, 1), 1: big.NewRat(1, 1), 9: big.NewRat(-1, 1)}
}
func TestExactCurveQuadricAlgebraicNonlinear(t *testing.T) {
	c := sharedProjectionCurve(t, []Vec3{{0, 0, 0}, {0, .5, 0}, {1, 1, 0}}, nil)
	r, e := IntersectExactCurveQuadric(context.Background(), c, circleQuadric(), ExactCurveQuadricOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 {
		t.Fatal(r, e)
	}
	h := r.Hits[0]
	if h.Parameter != nil || h.RationalDiagnostics || h.Multiplicity != 1 {
		t.Fatal(h)
	}
	f := func(x *big.Rat) *big.Rat {
		x2 := new(big.Rat).Mul(x, x)
		return new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(x2, x2), x2), big.NewRat(1, 1))
	}
	if f(h.Lower).Sign() >= 0 || f(h.Upper).Sign() <= 0 {
		t.Fatal(h)
	}
	// Plane filtering excludes every apparent XY conic root at a nonzero height.
	lifted, e := c.Transform(Translation(Vec3{Z: math.Ldexp(1, -80)}), SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane := [4]*big.Rat{2: big.NewRat(1, 1)}
	r, e = IntersectExactCurveQuadric(context.Background(), lifted, circleQuadric(), ExactCurveQuadricOptions{Plane: &plane})
	if e != nil || !r.Complete || len(r.Hits) != 0 {
		t.Fatal(r, e)
	}
	_, e = IntersectExactCurveQuadric(context.Background(), c, circleQuadric(), ExactCurveQuadricOptions{ExactCurveSegmentOptions: ExactCurveSegmentOptions{MaxOperations: 1}})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
func TestExactCurveQuadricTangencyAndSingularity(t *testing.T) {
	ctx := context.Background()
	tangent := sharedProjectionCurve(t, []Vec3{{1, -1, 0}, {1, 1, 0}}, nil)
	r, e := IntersectExactCurveQuadric(ctx, tangent, circleQuadric(), ExactCurveQuadricOptions{})
	if e != nil || len(r.Hits) != 1 || !r.Hits[0].Tangent || r.Hits[0].Multiplicity != 2 || r.Hits[0].Parameter.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(r, e)
	}
	singular := sharedProjectionCurve(t, []Vec3{{-1, 0, 0}, {1, 0, 0}}, nil)
	q := ExactQuadric{0: big.NewRat(1, 1), 1: big.NewRat(1, 1)}
	r, e = IntersectExactCurveQuadric(ctx, singular, q, ExactCurveQuadricOptions{})
	if e != nil || len(r.Hits) != 1 || !r.Hits[0].QuadricSingular || r.Hits[0].Tangent {
		t.Fatal(r, e)
	}
	crossing := sharedProjectionCurve(t, []Vec3{{0, 0, 0}, {0, .5, 0}, {1, 1, 0}}, nil)
	q = ExactQuadric{3: big.NewRat(1, 1), 9: big.NewRat(-1, 1)}
	r, e = IntersectExactCurveQuadric(ctx, crossing, q, ExactCurveQuadricOptions{})
	if e != nil || len(r.Hits) != 1 || !r.Hits[0].Transverse || r.Hits[0].Parameter.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(r, e)
	}
}
func TestExactCurveQuadricCoincidentRationalArc(t *testing.T) {
	// Exact rational unit circle parameterization (1-t²,2t)/(1+t²).
	c := sharedProjectionCurve(t, []Vec3{{1, 0, 0}, {1, 1, 0}, {0, 1, 0}}, []float64{1, 1, 2})
	p, e := c.patch.InsertUKnot(.375, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	c = &ExactSplineCurve{patch: p}
	plane := [4]*big.Rat{2: big.NewRat(1, 1)}
	r, e := IntersectExactCurveQuadric(context.Background(), c, circleQuadric(), ExactCurveQuadricOptions{Plane: &plane})
	if e != nil || !r.Complete || len(r.Hits) != 0 || len(r.Coincident) != 2 {
		t.Fatal(r, e)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = IntersectExactCurveQuadric(canceled, c, circleQuadric(), ExactCurveQuadricOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	_, e = IntersectExactCurveQuadric(context.Background(), c, ExactQuadric{}, ExactCurveQuadricOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
