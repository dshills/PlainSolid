package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicExactNonuniformCurveQueriesAtInsertedStation(t *testing.T) {
	ctx := context.Background()
	source, err := NewPeriodicBSpline(1, []Vec3{{0, 0, 0}, {1, 0, 0}, {0, 2, 0}}, nil, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	c, err := source.ExactCyclicSpline(ctx, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c, err = c.InsertKnot(ctx, big.NewRat(1, 6), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cycle := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 100))
	station := func(q *big.Rat) *big.Rat { return new(big.Rat).Add(new(big.Rat).SetInt(cycle), q) }
	start, edit, end := station(big.NewRat(1, 12)), station(big.NewRat(1, 6)), station(big.NewRat(1, 4))
	plane, err := NewPlane(Vec3{X: .5}, Vec3{X: 1})
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.IntersectPlaneExact(ctx, plane, start, end, PeriodicSplinePlaneOptions{})
	if err != nil || len(out.Hits) != 2 || len(out.Coincident) != 0 {
		t.Fatal("the two closed incidences of the new knot were lost", out, err)
	}
	for i, hit := range out.Hits {
		root := hit.Parameter.Interval()
		if hit.Cycle.Cmp(cycle) != 0 || hit.Span != i || hit.Multiplicity != 1 || root.Lower.Cmp(edit) != 0 || root.Upper.Cmp(edit) != 0 {
			t.Fatal("wrong exact knot incidence", i, hit)
		}
	}
	q := [3]*big.Rat{big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 7)}
	d, err := c.BoundPointDistanceRational(ctx, q, start, end, PeriodicCurveDistanceOptions{})
	if err != nil || !d.Complete || !d.HasWitness || d.OriginalParameter.Cmp(edit) != 0 || d.Cycle.Cmp(cycle) != 0 || d.Span != 1 ||
		d.Squared.Lower.Cmp(big.NewRat(1, 49)) != 0 || d.Squared.Upper.Cmp(big.NewRat(1, 49)) != 0 {
		t.Fatal("wrong exact nearest point or right owner", d, err)
	}
	m, err := c.TessellateCertifiedWindow(ctx, start, end, PeriodicTessellationOptions{
		CertifiedTessellationOptions: CertifiedTessellationOptions{Deflection: 0},
	})
	if err != nil || !m.Complete || len(m.Segments) != 2 {
		t.Fatal("linear tessellation should retain exactly the inserted station", m, err)
	}
	a, b := m.Segments[0], m.Segments[1]
	if a.Domain.Lower.Cmp(start) != 0 || a.Domain.Upper.Cmp(edit) != 0 || b.Domain.Lower.Cmp(edit) != 0 || b.Domain.Upper.Cmp(end) != 0 ||
		a.ErrorUpper != 0 || b.ErrorUpper != 0 || a.Span != 0 || b.Span != 1 || a.End != (Vec3{X: .5}) || a.End != b.Start {
		t.Fatal("linear chord, deflection or original station changed", m)
	}
}

func TestPeriodicExactNonuniformSurfaceQueries(t *testing.T) {
	ctx := context.Background()
	s, err := periodicDistanceSurface(t).ExactCyclicSurface(ctx, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.InsertPeriodicKnot(ctx, 0, big.NewRat(1, 6), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cycle := new(big.Int).Lsh(big.NewInt(1), 100)
	station := func(q *big.Rat) *big.Rat { return new(big.Rat).Add(new(big.Rat).SetInt(cycle), q) }
	u := ExactParameterInterval{Lower: station(big.NewRat(-1, 10)), Upper: station(big.NewRat(1, 3))}
	v := ExactParameterInterval{Lower: big.NewRat(-1, 2), Upper: big.NewRat(2, 3)}
	q := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 7), big.NewRat(5, 2)}
	d, err := s.BoundPointDistanceRational(ctx, q, u, v, PeriodicSurfaceDistanceOptions{})
	if err != nil || !d.Complete || !d.HasUpperPoint || d.OriginalUV[0].Cmp(station(big.NewRat(1, 6))) != 0 ||
		d.OriginalUV[1].Cmp(big.NewRat(-1, 8)) != 0 || d.U.Cycle.Cmp(cycle) != 0 ||
		d.Squared.Lower.Cmp(big.NewRat(1, 49)) != 0 || d.Squared.Upper.Cmp(big.NewRat(1, 49)) != 0 {
		t.Fatal("wrong exact witness on the newly inserted knot", d, err)
	}
	assertRationalParameterBracket(t, d.U.Domain, station(big.NewRat(1, 6)))
	u.Upper = station(big.NewRat(2, 3))
	parameter := ExactParameterInterval{Lower: big.NewRat(1, 11), Upper: big.NewRat(31, 6)}
	o := [3]*big.Rat{big.NewRat(1, 5), big.NewRat(-1, 4), big.NewRat(5, 2)}
	w := [3]*big.Rat{new(big.Rat), big.NewRat(3, 7), new(big.Rat)}
	h, err := s.IntersectRationalLine(ctx, o, w, u, v, parameter, PeriodicSurfaceLineOptions{})
	if err != nil || !h.Complete || len(h.Hits) != 2 || len(h.Pending) != 0 {
		t.Fatal("did not certify both original transverse line contacts", h, err)
	}
	for i, hit := range h.Hits {
		wantU := []*big.Rat{big.NewRat(1, 30), big.NewRat(19, 30)}[i]
		wantLine := []*big.Rat{big.NewRat(7, 12), big.NewRat(287, 60)}[i]
		if hit.U.Span != 2*i || hit.U.Cycle.Cmp(cycle) != 0 || hit.V.Cycle != nil {
			t.Fatal("contact lost original phase or refined span identity", hit)
		}
		assertRationalParameterBracket(t, hit.Isolation.U, station(wantU))
		assertRationalParameterBracket(t, hit.Isolation.V, big.NewRat(-1, 8))
		assertRationalParameterBracket(t, hit.Isolation.Parameter, wantLine)
	}
	short := PeriodicSurfaceLineOptions{PeriodicSurfaceBezierOptions: PeriodicSurfaceBezierOptions{MaxCells: 1}}
	if out, err := s.IntersectRationalLine(ctx, o, w, u, v, parameter, short); !errors.Is(err, ErrConvergence) || out.Complete || len(out.Hits) != 0 || len(out.Pending) != 1 || out.Pending[0].U.Span != -1 {
		t.Fatal("unbuilt knot grid did not preserve the pending original search", out, err)
	}
}
