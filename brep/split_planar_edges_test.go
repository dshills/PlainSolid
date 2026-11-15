package brep

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestSplitPlanarEdgesRetainsExactParentIntervals(t *testing.T) {
	s, err := Box(geom.Vec3{}, geom.Vec3{X: 3, Y: 1, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	id := -1
	for i, e := range s.Edges() {
		a, b := s.Vertices()[e.Start], s.Vertices()[e.End]
		if a == (geom.Vec3{}) && b == (geom.Vec3{X: 3}) {
			id = i
			break
		}
	}
	if id < 0 {
		t.Fatal("fixture does not have a three-unit outward edge")
	}
	ctx := context.Background()
	x1, x2 := geom.Vec3{X: 1}, geom.Vec3{X: 2}
	out, h, err := SplitPlanarEdges(ctx, s, []PlanarEdgeSplit{{Edge: id, Points: []geom.Vec3{x2, x1}}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	c := h.Children[id]
	ends := [][2]string{{"0", "1/3"}, {"1/3", "2/3"}, {"2/3", "1"}}
	if len(c) != 3 || c[0].Edge != id || len(out.Faces()) != len(s.Faces()) || !reflect.DeepEqual(out.Vertices()[:8], s.Vertices()) {
		t.Fatalf("original planar incidence was lost: %+v", h)
	}
	for i, z := range c {
		for j, r := range z.Range {
			if r.RatString() != ends[i][j] {
				t.Fatalf("inexact original range: %v", z.Range)
			}
		}
	}
	m, err := out.MassProperties()
	if err != nil || math.Abs(m.Volume-3) > 1e-13 || math.Abs(m.SurfaceArea-14) > 1e-13 {
		t.Fatalf("subdivision changed stock: %+v %v", m, err)
	}
	again, _, err := SplitPlanarEdges(ctx, s, []PlanarEdgeSplit{{Edge: id, Points: []geom.Vec3{x1, x2}}}, 2)
	if err != nil || !reflect.DeepEqual(out, again) {
		t.Fatal("input station order changed the native solid", err)
	}
	for _, tc := range []struct {
		p   []geom.Vec3
		max int
		err error
	}{
		{[]geom.Vec3{x1, x2}, 1, ErrTriangulationLimit},
		{[]geom.Vec3{x1, x1}, 2, geom.ErrInvalid},
		{[]geom.Vec3{{X: 1, Y: 1.0 / 1024}}, 1, geom.ErrDomain},
		{[]geom.Vec3{{X: 3}}, 1, geom.ErrDomain},
	} {
		out, h, err := SplitPlanarEdges(ctx, s, []PlanarEdgeSplit{{Edge: id, Points: tc.p}}, tc.max)
		if !errors.Is(err, tc.err) || len(out.Faces()) != 0 || h.Children != nil {
			t.Fatalf("unproved station published topology: %+v %v", h, err)
		}
	}
}
