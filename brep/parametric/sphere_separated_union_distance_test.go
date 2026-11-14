package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestNativeSphereSeparatedUnionNearest(t *testing.T) {
	ctx := context.Background()
	a := Ball{Radius: 2}
	b := Ball{Center: geom.Vec3{X: 6}, Radius: 1}
	s, e := NewSpherePairSet(ctx, a, b, Union, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	r := big.NewRat
	p := [3]*big.Rat{r(7, 2), r(0, 1), r(0, 1)}
	g, e := s.NearestSeparatedUnionBoundaryRational(ctx, p, geom.SphereFullBoundaryDistanceOptions{})
	if e != nil || len(g.TiedFaces) != 2 || g.Geometry.Lower.Cmp(r(9, 4)) != 0 {
		t.Fatal(g, e)
	}
	for _, f := range s.Solid().Faces() {
		if f.Reversed {
			t.Fatal("outward union")
		}
	}
	m, e := s.Solid().MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(m.Volume-12*math.Pi) > 1e-6 {
		t.Fatal(m, e)
	}
	g.TiedFaces[0] = 999
	again, e := s.NearestSeparatedUnionBoundaryRational(ctx, p, geom.SphereFullBoundaryDistanceOptions{})
	if e != nil || again.TiedFaces[0] == 999 {
		t.Fatal("owned results")
	}
	b.Center.X = 3
	if _, e := NewSpherePairSet(ctx, a, b, Union, 1e-9); e == nil {
		t.Fatal("native tangent gate changed")
	}
}
