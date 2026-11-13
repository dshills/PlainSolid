package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestIntersectSplineFaceLineWithHole(t *testing.T) {
	ctx := context.Background()
	m := func(z float64) *geom.BSplineSurface {
		t.Helper()
		k := []float64{0, 0, 1, 1}
		p := []geom.Vec3{{Z: z}, {Y: 1, Z: z}, {X: 1, Z: z}, {X: 1, Y: 1, Z: z + .25}}
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	k := geom.ParameterRectangle{UMax: 1, VMax: 1}
	s, _, e := assembleOpenSplineGraphPair(m(1), m(0), 0, true, 1e-9, partitionGraphRectangleLoop(k))
	if e != nil {
		t.Fatal(e)
	}
	outer, e := ellipseGraphContactLoop(geom.GraphEllipseHint{Center: [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)},
		RadiusSquared: [2]*big.Rat{big.NewRat(1, 64), big.NewRat(1, 64)}})
	if e != nil {
		t.Fatal(e)
	}
	curves := make([]PCurve, len(outer))
	for i, x := range outer {
		curves[i] = x.c
	}
	s, h, e := s.SplitFaceCurvedLoopContext(ctx, 0, curves)
	if e != nil {
		t.Fatal(e)
	}
	o := SplineFaceLineOptions{Support: geom.LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 4}}
	for i, p := range []geom.Vec3{{X: .25, Y: .25, Z: 3}, {X: .5, Y: .5, Z: 3}, {X: .62, Y: .62, Z: 3}} {
		l, e := geom.NewLine(p, geom.Vec3{Z: -1})
		if e != nil {
			t.Fatal(e)
		}
		r, e := s.IntersectSplineFaceLine(ctx, h.Outer, l, o)
		want := 1
		if i == 1 {
			want = 0
		}
		if e != nil || !r.Complete || len(r.Hits) != want || len(r.Excluded) != 1-want {
			t.Fatal(r, e)
		}
	}
	box := geom.UVBox{Min: geom.Vec2{X: .61, Y: .61}, Max: geom.Vec2{X: .62, Y: .62}}
	loc, proved, e := classifySplineFaceRootBox(ctx, s.faces[h.Outer], box, o.Trim, 256)
	if e != nil || !proved || loc != trimvalidate.Inside {
		t.Fatal(loc, proved, e)
	}
	if _, proved, e := classifySplineFaceRootBox(ctx, s.faces[h.Outer], box, o.Trim, 1); e != nil || proved {
		t.Fatal("budget must retain a root whose trim hull is not excluded", proved, e)
	}

	l, e := geom.NewLine(geom.Vec3{X: .625, Y: .5, Z: 3}, geom.Vec3{Z: -1})
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.IntersectSplineFaceLine(ctx, h.Outer, l, o)
	if e == nil && !r.Complete || e != nil && !errors.Is(e, geom.ErrConvergence) {
		t.Fatal(r, e)
	}
	if !r.Complete && len(r.Unresolved) == 0 && len(r.Support.Pending) == 0 {
		t.Fatal("missing trim boundary evidence", r)
	}
}
