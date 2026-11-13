package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func rationalPoint(x, y, z int64) [3]*big.Rat {
	return [3]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1), big.NewRat(z, 1)}
}
func TestExactSpatialCurveSegment(t *testing.T) {
	s, _ := NewBezier([]Vec3{{}, {X: 1}, {X: 2, Y: 1}, {X: 3, Y: 3, Z: 1}}, nil)
	c := isoIntersectionFixture(t, s)
	start, end := rationalPoint(1, -1, 0), rationalPoint(1, 1, 0)
	start[2] = big.NewRat(1, 27)
	end[2] = big.NewRat(1, 27)
	r, e := IntersectExactCurveSegment(context.Background(), c, start, end, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 {
		t.Fatal(r, e)
	}
	hit := r.Hits[0]
	if hit.Parameter.RatString() != "1/3" || hit.SegmentParameter.RatString() != "2/3" || !hit.Transverse || hit.Point[2].RatString() != "1/27" {
		t.Fatal(hit)
	}
	r, e = IntersectExactCurveSegment(context.Background(), c, end, start, ExactCurveSegmentOptions{})
	if e != nil || r.Hits[0].SegmentParameter.RatString() != "1/3" {
		t.Fatal(r, e)
	}
	r, e = IntersectExactCurveSegment(context.Background(), c, rationalPoint(0, 0, 0), rationalPoint(0, 1, 0), ExactCurveSegmentOptions{})
	if e != nil || len(r.Hits) != 1 || !r.Hits[0].SharedEndpoint || !r.Hits[0].Transverse {
		t.Fatal(r, e)
	}
}
func TestExactCurveSegmentNearSkewAndTangency(t *testing.T) {
	ctx := context.Background()
	a, b := rationalPoint(0, 0, 0), rationalPoint(1, 0, 0)
	s, _ := NewBezier([]Vec3{{Y: -.5, Z: -.5}, {X: 1, Y: .5, Z: .5}}, nil)
	c := isoIntersectionFixture(t, s)
	shifted, _ := c.Transform(Translation(Vec3{Z: math.Ldexp(1, -80)}), SurfacePatchOptions{})
	r, e := IntersectExactCurveSegment(ctx, shifted, a, b, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 0 {
		t.Fatal("near skew", r, e)
	}
	s, _ = NewBezier([]Vec3{{Y: .25}, {X: .5, Y: -.25}, {X: 1, Y: .25}}, nil)
	r, e = IntersectExactCurveSegment(ctx, isoIntersectionFixture(t, s), a, b, ExactCurveSegmentOptions{})
	if e != nil || len(r.Hits) != 1 || !r.Hits[0].Tangent || r.Hits[0].Multiplicity != 2 {
		t.Fatal(r, e)
	}
	s, _ = NewBezier([]Vec3{{X: -1, Y: -1}, {X: 2, Y: 2}, {X: -4, Y: -4}, {X: 8, Y: 8}}, nil)
	r, e = IntersectExactCurveSegment(ctx, isoIntersectionFixture(t, s), a, b, ExactCurveSegmentOptions{})
	if e != nil || len(r.Hits) != 1 || !r.Hits[0].Singular || r.Hits[0].Multiplicity != 3 {
		t.Fatal(r, e)
	}
}
func TestExactCurveSegmentAlgebraicCoincidentAndLimits(t *testing.T) {
	ctx := context.Background()
	a, b := rationalPoint(0, 0, 0), rationalPoint(1, 0, 0)
	s, _ := NewBezier([]Vec3{{Y: -.5}, {X: .5, Y: -.5}, {X: 1, Y: .5}}, nil)
	c := isoIntersectionFixture(t, s)
	r, e := IntersectExactCurveSegment(ctx, c, a, b, ExactCurveSegmentOptions{})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Pending) != 1 {
		t.Fatal(r, e)
	}
	half := rationalPoint(0, 0, 0)
	half[0] = big.NewRat(1, 2)
	r, e = IntersectExactCurveSegment(ctx, c, a, half, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 0 {
		t.Fatal(r, e)
	}
	s, _ = NewBezier([]Vec3{{}, {X: 1}}, nil)
	line := isoIntersectionFixture(t, s)
	r, e = IntersectExactCurveSegment(ctx, line, a, b, ExactCurveSegmentOptions{})
	if !errors.Is(e, ErrConvergence) || len(r.Coincident) != 1 {
		t.Fatal(r, e)
	}
	r, e = IntersectExactCurveSegment(ctx, line, rationalPoint(2, 0, 0), rationalPoint(3, 0, 0), ExactCurveSegmentOptions{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	_, e = IntersectExactCurveSegment(ctx, c, a, a, ExactCurveSegmentOptions{})
	if !errors.Is(e, ErrDegenerate) {
		t.Fatal(e)
	}
	_, e = IntersectExactCurveSegment(ctx, c, a, b, ExactCurveSegmentOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = IntersectExactCurveSegment(canceled, c, a, b, ExactCurveSegmentOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestExactCurveAffineLineDifferentDegreeWeightsAndDomain(t *testing.T) {
	ctx := context.Background()
	s, _ := NewBezier([]Vec3{{Y: -1}, {X: 3, Y: 1}}, nil)
	curve := isoIntersectionFixture(t, s)
	// W=(1+t)/2, X=3t(1+t)/2; dyadic normalized weights give exactly X/W=3t.
	l, _ := NewBezier([]Vec3{{}, {X: 1}, {X: 3}}, []float64{2, 3, 4})
	line := isoIntersectionFixture(t, l)
	line, e := line.Reparameterize(-2, 4, false, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := IntersectExactCurveAffineLine(ctx, curve, line, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 || r.Hits[0].OtherParameter.RatString() != "1" || !r.Hits[0].Transverse {
		t.Fatal(r, e)
	}
	nonlinear, _ := NewBezier([]Vec3{{}, {X: 1}}, []float64{1, 2})
	r, e = IntersectExactCurveAffineLine(ctx, curve, isoIntersectionFixture(t, nonlinear), ExactCurveSegmentOptions{})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	_, e = IntersectExactCurveAffineLine(ctx, curve, line, ExactCurveSegmentOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}

func TestExactCurveSegmentAffineAndKnotInvariance(t *testing.T) {
	s, _ := NewBezier([]Vec3{{}, {X: 1}, {X: 2, Y: 1}, {X: 3, Y: 3, Z: 1}}, nil)
	curve := isoIntersectionFixture(t, s)
	start, end := rationalPoint(1, -1, 0), rationalPoint(1, 1, 0)
	start[2] = big.NewRat(1, 27)
	end[2] = big.NewRat(1, 27)
	refined, e := curve.patch.InsertUKnot(.5, 2, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := IntersectExactCurveSegment(context.Background(), &SurfaceIsoCurve{patch: refined}, start, end, ExactCurveSegmentOptions{})
	if e != nil || len(r.Hits) != 1 || r.Hits[0].Parameter.RatString() != "1/3" {
		t.Fatal(r, e)
	}
	for i := -5; i <= 5; i++ {
		m := Identity()
		m[0][0] = 2
		m[0][1] = float64(i)
		m[1][1] = -1
		m[1][2] = float64(i - 2)
		m[2][2] = 4
		m[0][3] = float64(i + 1)
		m[2][3] = float64(2 * i)
		transformed, e := curve.Transform(m, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		apply := func(p [3]*big.Rat) (q [3]*big.Rat) {
			for axis := 0; axis < 3; axis++ {
				q[axis] = new(big.Rat).SetFloat64(m[axis][3])
				for j := 0; j < 3; j++ {
					q[axis].Add(q[axis], new(big.Rat).Mul(new(big.Rat).SetFloat64(m[axis][j]), p[j]))
				}
			}
			return
		}
		r, e := IntersectExactCurveSegment(context.Background(), transformed, apply(start), apply(end), ExactCurveSegmentOptions{})
		if e != nil || len(r.Hits) != 1 || r.Hits[0].Parameter.RatString() != "1/3" || r.Hits[0].SegmentParameter.RatString() != "2/3" || !r.Hits[0].Transverse {
			t.Fatal(i, r, e)
		}
	}
}
