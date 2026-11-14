package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func bulgingAffineFace(t *testing.T, middle *big.Rat) Face {
	f := affineQueryFaceFixture(t)
	f.Holes = nil
	h := [3][3]*big.Rat{{big.NewRat(1, 4), big.NewRat(1, 4), big.NewRat(1, 1)}, {middle, big.NewRat(1, 2), big.NewRat(1, 1)}, {big.NewRat(1, 4), big.NewRat(3, 4), big.NewRat(1, 1)}}
	curve, e := NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	_, ends := curve.Endpoints()
	p := []geom.Vec2{ends[0], ends[1], {X: 0, Y: .75}, {X: 0, Y: .25}}
	f.Outer = nil
	for i, a := range p {
		f.Outer = append(f.Outer, Use{Edge: i, UVStart: a, UVEnd: p[(i+1)%len(p)]})
	}
	f.Outer[0].Curve = curve
	return f
}
func TestAffineFaceDomainSubdivision(t *testing.T) {
	ctx := context.Background()
	for _, middle := range []*big.Rat{big.NewRat(5, 4), big.NewRat(7, 4)} {
		f := bulgingAffineFace(t, middle)
		if q, e := NewAffineSplineFaceQuery(ctx, f, AffineSplineFaceQueryOptions{}); !errors.Is(e, geom.ErrDomain) || q != nil {
			t.Fatal(q, e)
		}
		q, e := NewAffineSplineFaceQuery(ctx, f, AffineSplineFaceQueryOptions{MaxDomainSubdivisions: 1})
		if e != nil || q == nil || q.DomainSubdivisions() != 1 {
			t.Fatal(q, e)
		}
		// Exact maximum u=(1/4+middle)/2 at t=1/2, v=1/2.
		u := new(big.Rat).Quo(new(big.Rat).Add(big.NewRat(1, 4), middle), big.NewRat(2, 1))
		point := [3]*big.Rat{new(big.Rat).Mul(big.NewRat(2, 1), u), big.NewRat(3, 2), new(big.Rat).Add(u, big.NewRat(1, 2))}
		r, e := q.ClassifyRationalPoint(ctx, point, geom.AffineChartInverseOptions{})
		if e != nil || !r.Proven || r.Location != trimvalidate.Boundary {
			t.Fatal(r, e)
		}
	}
	exit := bulgingAffineFace(t, big.NewRat(15, 8))
	q, e := NewAffineSplineFaceQuery(ctx, exit, AffineSplineFaceQueryOptions{MaxDomainSubdivisions: 64})
	if !errors.Is(e, geom.ErrDomain) || q != nil {
		t.Fatal(q, e)
	}
	// Two individually valid interior bulges consume a shared face budget.
	paired := affineQueryFaceFixture(t)
	paired.Holes = nil
	paired.Outer = nil
	for i, m := range []*big.Rat{big.NewRat(5, 4), big.NewRat(-1, 4)} {
		a, z := big.NewRat(1, 4), big.NewRat(3, 4)
		if i == 1 {
			a, z = z, a
		}
		curve, e := NewExactQuadraticUV([3][3]*big.Rat{{big.NewRat(1, 2), a, big.NewRat(1, 1)}, {m, big.NewRat(1, 2), big.NewRat(1, 1)}, {big.NewRat(1, 2), z, big.NewRat(1, 1)}})
		if e != nil {
			t.Fatal(e)
		}
		_, ends := curve.Endpoints()
		paired.Outer = append(paired.Outer, Use{Edge: i, Curve: curve, UVStart: ends[0], UVEnd: ends[1]})
	}
	if q, e := NewAffineSplineFaceQuery(ctx, paired, AffineSplineFaceQueryOptions{MaxDomainSubdivisions: 1}); !errors.Is(e, geom.ErrConvergence) || q != nil {
		t.Fatal(q, e)
	}
	// Non-dyadic tangency is not accepted from interior samples. At depth1
	// an enclosing Bernstein hull remains outside: this is unresolved, not exit.
	h := [][3]*big.Rat{{big.NewRat(8, 9), big.NewRat(1, 4), big.NewRat(1, 1)}, {big.NewRat(11, 9), big.NewRat(1, 2), big.NewRat(1, 1)}, {big.NewRat(5, 9), big.NewRat(3, 4), big.NewRat(1, 1)}}
	b, _ := newFaceDomainBudget(1, 1)
	bounds := [2][2]*big.Rat{{new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 1)}}
	if e := b.prove(ctx, h, bounds, 4096); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal(e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := NewAffineSplineFaceQuery(cancelled, bulgingAffineFace(t, big.NewRat(5, 4)), AffineSplineFaceQueryOptions{MaxDomainSubdivisions: 1}); !errors.Is(e, context.Canceled) || q != nil {
		t.Fatal(q, e)
	}
}
