package fillet

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func rectangle(w, h float64) []geom.Vec2 { return []geom.Vec2{{}, {X: w}, {X: w, Y: h}, {Y: h}} }
func TestRoundedRectangleAnalyticAndMesh(t *testing.T) {
	r, e := Prism(context.Background(), rectangle(4, 3), 2, 5, .4, Options{})
	if e != nil {
		t.Fatal(e)
	}
	area := 12 - (4-math.Pi)*.16
	perimeter := 14 - 8*.4 + 2*math.Pi*.4
	if math.Abs(r.NominalArea-area) > 1e-13 || math.Abs(r.NominalPerimeter-perimeter) > 1e-13 {
		t.Fatal(r.NominalArea, r.NominalPerimeter)
	}
	p, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(p.Volume-3*area) > 1e-7 || math.Abs(p.Area-(2*area+3*perimeter)) > 1e-7 || p.Centroid.Sub(geom.Vec3{X: 2, Y: 1.5, Z: 3.5}).Length() > 1e-8 {
		t.Fatal(p)
	}
	cylinders := 0
	for _, f := range r.Solid.Faces() {
		if _, ok := f.Surface.(geom.CylinderSurface); ok {
			cylinders++
		}
	}
	if cylinders != 4 || len(r.Solid.Faces()) != 10 {
		t.Fatal("not analytic cylinder topology")
	}
	m, e := r.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .03, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if math.Abs(m.SignedVolume()-3*area) > .15 {
		t.Fatal("tessellation volume", m.SignedVolume())
	}
	report, e := collision.SelfIntersections(m, collision.Options{MaxPairs: 2000000, MaxContacts: 1})
	if e != nil || len(report.Contacts) > 0 {
		t.Fatal("nonembedded tessellation", e, report.Contacts)
	}
}
func TestTriangleAndPentagon(t *testing.T) {
	for _, n := range []int{3, 5} {
		var polygon []geom.Vec2
		for i := 0; i < n; i++ {
			a := 2 * math.Pi * float64(i) / float64(n)
			polygon = append(polygon, geom.Vec2{X: 3 * math.Cos(a), Y: 3 * math.Sin(a)})
		}
		r, e := Prism(context.Background(), polygon, -1, 1, .2, Options{})
		if e != nil {
			t.Fatal(e)
		}
		theta := 2 * math.Pi / float64(n)
		area := float64(n)*9*math.Sin(theta)/2 - float64(n)*.04*(math.Tan(theta/2)-theta/2)
		p, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(p.Volume-2*area) > 1e-7 || math.Abs(p.Centroid.Z) > 1e-8 {
			t.Fatal(p, area)
		}
		for _, c := range r.Corners {
			if math.Abs(c.Sweep-theta) > 1e-14 {
				t.Fatal(c)
			}
		}
	}
}
func TestRadiusAndInputRejection(t *testing.T) {
	p := rectangle(4, 4)
	if _, e := Prism(context.Background(), p, 0, 1, 1.1, Options{}); !errors.Is(e, ErrRadius) {
		t.Fatal("expected conservative disk limit", e)
	}
	if _, e := Prism(context.Background(), p, 0, 1, 2.1, Options{}); !errors.Is(e, ErrRadius) {
		t.Fatal(e)
	}
	bad := []geom.Vec2{{}, {X: 3}, {X: 1, Y: 1}, {X: 3, Y: 3}, {Y: 3}}
	if _, e := Prism(context.Background(), bad, 0, 1, .1, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	for _, o := range []Options{{MaxCorners: 3}, {MaxWork: 1}} {
		if _, e := Prism(context.Background(), p, 0, 1, .1, o); !errors.Is(e, ErrLimit) {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Prism(ctx, p, 0, 1, .1, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestRotatedRectangleTangencyAndScale(t *testing.T) {
	p := rectangle(4, 3)
	c, s := math.Cos(.37), math.Sin(.37)
	for i, v := range p {
		p[i] = geom.Vec2{X: 10 + c*v.X - s*v.Y, Y: -5 + s*v.X + c*v.Y}
	}
	r, e := Prism(context.Background(), p, 0, 2, .3, Options{})
	if e != nil {
		t.Fatal(e)
	}
	props, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	want := 2 * (12 - (4-math.Pi)*.09)
	if math.Abs(props.Volume-want) > 1e-7 {
		t.Fatal(props.Volume, want)
	}
	for i, corner := range r.Corners {
		next := r.Corners[(i+1)%len(p)]
		edge := sub(next.Incoming, corner.Outgoing)
		if math.Abs(dot(sub(corner.Outgoing, corner.Center), edge))/math.Hypot(edge.X, edge.Y) > 1e-9 {
			t.Fatal("not tangent")
		}
	}
	for i := range p {
		p[i] = scale(p[i], 2)
	}
	rr, e := Prism(context.Background(), p, 0, 4, .6, Options{})
	if e != nil || math.Abs(rr.NominalVolume-8*r.NominalVolume) > 1e-10 {
		t.Fatal(e)
	}
}
func TestPentagonMeshConformity(t *testing.T) {
	var p []geom.Vec2
	for i := 0; i < 5; i++ {
		a := 2 * math.Pi * float64(i) / 5
		p = append(p, geom.Vec2{X: 3 * math.Cos(a), Y: 3 * math.Sin(a)})
	}
	r, e := Prism(context.Background(), p, 0, 1, .2, Options{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .04, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	report, e := collision.SelfIntersections(m, collision.Options{MaxPairs: 2000000, MaxContacts: 1})
	if e != nil || len(report.Contacts) != 0 {
		t.Fatal(e, report.Contacts)
	}
}

func FuzzRegularPolygonAnalyticVolume(f *testing.F) {
	f.Add(uint8(3), uint8(4), int8(7))
	f.Fuzz(func(t *testing.T, count, size uint8, angle int8) {
		n := 3 + int(count%6)
		outer := 2 + float64(size%8)/2
		rotation := float64(angle) / 100
		var p []geom.Vec2
		for i := 0; i < n; i++ {
			a := rotation + 2*math.Pi*float64(i)/float64(n)
			p = append(p, geom.Vec2{X: 1 + outer*math.Cos(a), Y: -2 + outer*math.Sin(a)})
		}
		radius := .1
		r, e := Prism(context.Background(), p, 0, 1, radius, Options{})
		if e != nil {
			t.Fatal(e)
		}
		theta := 2 * math.Pi / float64(n)
		want := float64(n)*outer*outer*math.Sin(theta)/2 - float64(n)*radius*radius*(math.Tan(theta/2)-theta/2)
		props, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(props.Volume-want) > 1e-7*want {
			t.Fatal(props.Volume, want)
		}
	})
}
