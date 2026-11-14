package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func affineContactPatch(t *testing.T, f func(float64, float64) geom.Vec3, domain geom.ParameterRectangle, refined bool) Patch {
	t.Helper()
	stations := []float64{0, 1}
	knots := []float64{domain.UMin, domain.UMin, domain.UMax, domain.UMax}
	if refined {
		stations = []float64{0, .5, 1}
		knots = []float64{domain.UMin, domain.UMin, (domain.UMin + domain.UMax) / 2, domain.UMax, domain.UMax}
	}
	poles := []geom.Vec3{}
	for _, u := range stations {
		for _, v := range []float64{0, 1} {
			poles = append(poles, f(u, v))
		}
	}
	s, e := geom.NewBSplineSurface(1, 1, len(stations), 2, poles, knots, []float64{domain.VMin, domain.VMin, domain.VMax, domain.VMax}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return Patch{Surface: s, Min: geom.Vec2{X: domain.UMin, Y: domain.VMin}, Max: geom.Vec2{X: domain.UMax, Y: domain.VMax}}
}
func TestOriginalAffinePatchContact(t *testing.T) {
	ctx := context.Background()
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	embed := func(x, y float64) geom.Vec3 { return geom.Vec3{X: x, Y: y, Z: x + 2*y} }
	a := affineContactPatch(t, embed, d, true)
	b := affineContactPatch(t, func(u, v float64) geom.Vec3 { return embed(3*u, u+v) }, geom.ParameterRectangle{UMin: 2, UMax: 4, VMin: -1, VMax: 2}, false)
	contact, e := IntersectAffinePatches(ctx, a, b, geom.SurfacePatchOptions{}, geom.AffineChartIntersectionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	kind, e := contact.Kind()
	if e != nil || kind != AffinePatchContactArea || !contact.Complete() {
		t.Fatal(kind, e)
	}
	area, squared, orientation, e := contact.CoplanarArea()
	if e != nil || area.Cmp(big.NewRat(5, 6)) != 0 || squared.Cmp(big.NewRat(25, 6)) != 0 || orientation != 1 {
		t.Fatal(area, squared, orientation, e)
	}
	vertices, _ := contact.Vertices()
	edges, _ := contact.Edges()
	if len(vertices) != 4 || len(edges) != 4 {
		t.Fatal(vertices, edges)
	}
	for i, edge := range edges {
		if edge.Start != i || edge.End != (i+1)%4 || edge.Boundary[0] == 0 && edge.Boundary[1] == 0 {
			t.Fatal(edge)
		}
	}
	seen := false
	for _, v := range vertices {
		if v.A[1].Cmp(big.NewRat(1, 3)) == 0 {
			seen = true
			if v.B[0].Cmp(big.NewRat(8, 3)) != 0 {
				t.Fatal(v)
			}
		}
	}
	if !seen {
		t.Fatal("nonbinary original parameter lost")
	}
	for i, want := range []Patch{a, b} {
		owner, e := contact.Owner(i)
		if e != nil || owner != want {
			t.Fatal(owner, e)
		}
	}
	vertices[0].A[0].SetInt64(99)
	vertices[0].Point[0].SetInt64(99)
	edges[0].Start = 99
	area.SetInt64(99)
	again, _ := contact.Vertices()
	againEdges, _ := contact.Edges()
	againArea, _, _, _ := contact.CoplanarArea()
	if again[0].A[0].Cmp(big.NewRat(99, 1)) == 0 || again[0].Point[0].Cmp(big.NewRat(99, 1)) == 0 || againEdges[0].Start == 99 || againArea.Cmp(big.NewRat(99, 1)) == 0 {
		t.Fatal("mutable publication")
	}
	for _, limit := range []int{contact.ExactOperations(), contact.ExactOperations() - 1} {
		q, e := IntersectAffinePatches(ctx, a, b, geom.SurfacePatchOptions{}, geom.AffineChartIntersectionOptions{MaxOperations: limit})
		if limit == contact.ExactOperations() {
			if e != nil || !q.Complete() {
				t.Fatal(e)
			}
		} else if e == nil || q != nil {
			t.Fatal("partial published contact")
		}
	}
	clipped := a
	clipped.Max.X = .5
	q, e := IntersectAffinePatches(ctx, clipped, b, geom.SurfacePatchOptions{}, geom.AffineChartIntersectionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	clippedArea, _, _, _ := q.CoplanarArea()
	if clippedArea.Cmp(big.NewRat(11, 24)) != 0 {
		t.Fatal("ignored original subdomain", clippedArea)
	}
}
func TestOriginalAffinePatchTransverseAndRejected(t *testing.T) {
	ctx := context.Background()
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	a := affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v} }, d, false)
	b := affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: .5, Y: u, Z: v - .5} }, d, false)
	q, e := IntersectAffinePatches(ctx, a, b, geom.SurfacePatchOptions{}, geom.AffineChartIntersectionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	kind, _ := q.Kind()
	edges, _ := q.Edges()
	if kind != AffinePatchContactSegment || len(edges) != 1 {
		t.Fatal(kind, edges)
	}
	if _, _, _, e = q.CoplanarArea(); e != geom.ErrDomain {
		t.Fatal("transverse gained area", e)
	}
	bent := affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v, Z: u * u} }, d, true)
	if q, e := IntersectAffinePatches(ctx, a, bent, geom.SurfacePatchOptions{}, geom.AffineChartIntersectionOptions{}); e == nil || q != nil {
		t.Fatal("nonaffine original source accepted")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := IntersectAffinePatches(cancelCtx, a, b, geom.SurfacePatchOptions{}, geom.AffineChartIntersectionOptions{}); e != context.Canceled || q != nil {
		t.Fatal(q, e)
	}
	var zero AffinePatchContact
	if zero.Complete() {
		t.Fatal("zero complete")
	}
	if _, e := zero.Vertices(); e != geom.ErrInvalid {
		t.Fatal(e)
	}
}

func TestOriginalAffinePatchClosedContactDimensions(t *testing.T) {
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	a := affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v} }, d, false)
	for _, tc := range []struct {
		x, y, z         float64
		kind            AffinePatchContactKind
		vertices, edges int
		coplanar        bool
	}{
		{2, 0, 0, AffinePatchContactEmpty, 0, 0, true},
		{1, 1, 0, AffinePatchContactPoint, 1, 0, true},
		{1, 0, 0, AffinePatchContactSegment, 2, 1, true},
		{0, 0, 1, AffinePatchContactEmpty, 0, 0, false},
	} {
		b := affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: u + tc.x, Y: v + tc.y, Z: tc.z} }, d, false)
		q, e := IntersectAffinePatches(context.Background(), a, b, geom.SurfacePatchOptions{}, geom.AffineChartIntersectionOptions{})
		if e != nil {
			t.Fatal(e)
		}
		kind, _ := q.Kind()
		vertices, _ := q.Vertices()
		edges, _ := q.Edges()
		if kind != tc.kind || len(vertices) != tc.vertices || len(edges) != tc.edges {
			t.Fatal(tc, kind, vertices, edges)
		}
		area, squared, _, e := q.CoplanarArea()
		if tc.coplanar {
			if e != nil || area.Sign() != 0 || squared.Sign() != 0 {
				t.Fatal(area, squared, e)
			}
		} else if e != geom.ErrDomain {
			t.Fatal(e)
		}
	}
}
