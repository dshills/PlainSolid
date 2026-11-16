package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSurfaceSurfaceProjectiveContactsRetainsExactIntersection(t *testing.T) {
	ctx := context.Background()
	a, b := projectiveTinyLinearHeightCharts(t)
	axes := [2]int{0, 1}
	o := SurfaceSurfaceIsolationOptions{ProjectiveGraphAxes: &axes, ProjectiveGraphContacts: true}
	for aOwner, pair := range [2][2]*SurfacePatch{{a, b}, {b, a}} {
		r, err := BoundSurfaceSurfaceContext(ctx, pair[0], pair[1], o)
		if err != nil || !r.Complete || r.VisitedPairs != 1 || r.ProjectiveContacts == nil || !r.ProjectiveContacts.Complete ||
			len(r.ProjectiveContacts.Contours) != 1 || len(r.ProjectiveContacts.Arcs) < 2 ||
			len(r.Candidates)+len(r.Pending)+len(r.Coincident) != 0 || r.ProjectiveCoincidence != nil {
			t.Fatal("complete original graph should replace candidate boxes in either order", err, r)
		}
		if r.ProjectiveContacts.Contours[0].Closed {
			t.Fatal("original separating trace should be open", r.ProjectiveContacts.Contours)
		}
		for _, vertex := range r.ProjectiveContacts.Vertices {
			assertProjectiveBoundaryRoot(t, vertex.OriginalUV[aOwner][0], big.NewRat(1, 2))
		}
	}
	for _, rotated := range []bool{false, true} {
		a, b, axes := projectiveQuadraticSaddleCharts(t, rotated)
		o.ProjectiveGraphAxes, o.ProjectiveGraphQuadraticSaddles = &axes, true
		r, err := BoundSurfaceSurfaceContext(ctx, a, b, o)
		if err != nil || !r.Complete || r.ProjectiveContacts == nil || len(r.ProjectiveContacts.SaddleVertices) != 1 ||
			len(r.ProjectiveContacts.Contours) != 4 || r.VisitedPairs != 1 {
			t.Fatal("pair query lost the certified original quadratic saddle", err, r)
		}
		g := r.ProjectiveContacts
		id := g.SaddleVertices[0]
		assertProjectiveBoundaryRoot(t, g.Vertices[id].Common[1], big.NewRat(3, 4))
		if r.ExactOperations < g.Cover.ExactOperations+g.JoinOperations || r.ExactOperations > 250_000 {
			t.Fatal("complete contour work was not charged", r.ExactOperations)
		}
	}
}

func TestSurfaceSurfaceProjectiveContactsFallbackRemainsConservative(t *testing.T) {
	ctx := context.Background()
	axis := [2]int{0, 1}
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v + .125} })
	o := SurfaceSurfaceIsolationOptions{ProjectiveGraphAxes: &axis, ProjectiveGraphContacts: true, ParameterTolerance: 32}
	r, err := BoundSurfaceSurfaceContext(ctx, a, b, o)
	if err != nil || !r.Complete || r.ProjectiveContacts == nil || len(r.ProjectiveContacts.Arcs) != 0 ||
		len(r.Candidates)+len(r.Pending) != 0 {
		t.Fatal("parallel separated lifts with overlapping AABBs should have an exact empty graph", err, r)
	}
	o.ProjectiveGraphOperations = 1
	r, err = BoundSurfaceSurfaceContext(ctx, a, b, o)
	if err != nil || !r.Complete || r.ProjectiveContacts != nil || len(r.Candidates) != 1 {
		t.Fatal("an exhausted accelerated proof must preserve the ordinary candidate", err, r)
	}
	a = arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	point, err := NewSurfacePatch(quadraticContactSurface(t, 0), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	o.ProjectiveGraphOperations, o.ProjectiveGraphQuadraticSaddles = 0, true
	r, err = BoundSurfaceSurfaceContext(ctx, a, point, o)
	if err != nil || !r.Complete || r.ProjectiveContacts != nil || len(r.Candidates) != 1 {
		t.Fatal("an unsupported isolated tangency still belongs to the conservative pair cover", err, r)
	}
	for _, bad := range []SurfaceSurfaceIsolationOptions{
		{ProjectiveGraphContacts: true},
		{ProjectiveGraphQuadraticSaddles: true, ProjectiveGraphAxes: &axis},
	} {
		if _, err := BoundSurfaceSurfaceContext(ctx, a, b, bad); !errors.Is(err, ErrInvalid) {
			t.Fatal("inconsistent projective pair options were accepted", err)
		}
	}
}
