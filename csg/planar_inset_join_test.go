package csg

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

func insetBox(t *testing.T, a, b geom.Vec3) brep.Solid {
	t.Helper()
	s, err := brep.Box(a, b)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestJoinPlanarInsetRetainsHostFaceAndOriginalToolCoedges(t *testing.T) {
	ctx := context.Background()
	host := insetBox(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 1})
	tool := insetBox(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 2})
	s, h, err := JoinPlanarInset(ctx, host, tool, 1, 0, collision.PlanarFaceContactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.MassProperties()
	p := h.Contact
	if err != nil || p.TwiceHostArea.RatString() != "32" || p.TwiceToolArea.RatString() != "8" ||
		p.SignedVolume.RatString() != "20" || len(s.Faces()) != 11 || len(s.Edges()) != 24 || len(s.Vertices()) != 16 ||
		math.Abs(m.SurfaceArea-56) > 1e-13 {
		t.Fatalf("wrong inset solid or source proof: %+v %+v %v", p, m, err)
	}
	f := s.Faces()[h.Owners[0].Faces[1]]
	if !reflect.DeepEqual(f.Boundary, host.Faces()[1].Boundary) || len(f.Holes) != 1 || h.Owners[1].Faces[0] != -1 {
		t.Fatalf("the host original face did not survive a single new hole: %+v", f)
	}
	for j, u := range tool.Faces()[0].Boundary {
		want := h.Owners[1].Edges[u.Edge]
		want.Reversed = u.Reversed
		if f.Holes[0][j] != want || p.ToolEdges[j] != u.Edge {
			t.Fatal("tool perimeter lost original coedge identity", j, want)
		}
	}
	loc, err := collision.NewPointLocator(ctx, s, collision.LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		p    geom.Vec3
		want collision.Location
	}{
		{geom.Vec3{X: 2, Y: 2, Z: 1}, collision.Inside},
		{geom.Vec3{X: 1, Y: 1, Z: 1}, collision.OnBoundary},
		{geom.Vec3{X: .5, Y: 2, Z: 1}, collision.OnBoundary},
		{geom.Vec3{X: .5, Y: 2, Z: 1.5}, collision.Outside},
	} {
		r, err := loc.Locate(ctx, tc.p)
		if err != nil || r.Location != tc.want {
			t.Fatalf("joint location at %v: %+v %v", tc.p, r, err)
		}
	}
	for _, low := range []geom.Vec3{{Y: 1, Z: 1}, {X: 3, Y: 1, Z: 1}, {X: 1, Y: 1, Z: .5}, {X: 1, Y: 1, Z: 3}} {
		other := insetBox(t, low, low.Add(geom.Vec3{X: 2, Y: 2, Z: 1}))
		o, r, err := JoinPlanarInset(ctx, host, other, 1, 0, collision.PlanarFaceContactOptions{})
		if !errors.Is(err, collision.ErrInvalid) || len(o.Faces()) != 0 || r.Contact.SignedVolume != nil {
			t.Fatalf("unproved inset at %v published material: %+v %v", low, r, err)
		}
	}
}

func TestJoinPlanarInsetPreservesExistingIndependentHostHole(t *testing.T) {
	ctx := context.Background()
	square := func(a, b float64) []geom.Vec3 {
		return []geom.Vec3{{X: a, Y: a}, {X: b, Y: a}, {X: b, Y: b}, {X: a, Y: b}}
	}
	host, err := brep.PrismWithHoles(square(0, 5), [][]geom.Vec3{square(3, 4)}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	tool := insetBox(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 2, Y: 2, Z: 2})
	s, h, err := JoinPlanarInset(ctx, host, tool, 1, 0, collision.PlanarFaceContactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	f := s.Faces()[h.Owners[0].Faces[1]]
	p, err := s.MassProperties()
	if err != nil || len(f.Holes) != 2 || !reflect.DeepEqual(f.Holes[0], host.Faces()[1].Holes[0]) ||
		h.Contact.SignedVolume.RatString() != "25" || math.Abs(p.SurfaceArea-76) > 1e-13 {
		t.Fatalf("second hole changed the original host trim: %+v %+v %v", f, p, err)
	}
	insideHole := insetBox(t, geom.Vec3{X: 3.125, Y: 3.125, Z: 1}, geom.Vec3{X: 3.875, Y: 3.875, Z: 2})
	if s, _, err := JoinPlanarInset(ctx, host, insideHole, 1, 0, collision.PlanarFaceContactOptions{}); !errors.Is(err, collision.ErrInvalid) || len(s.Faces()) != 0 {
		t.Fatalf("empty host hole was used as a contact surface: %v", err)
	}
}
