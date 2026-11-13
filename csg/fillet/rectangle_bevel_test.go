package fillet

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/collision"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestRoundedRectangleTopBevel(t *testing.T) {
	r, e := PrismRectangleTopBeveled(context.Background(), geom.Vec2{}, geom.Vec2{X: 6, Y: 4}, -1, 2, .75, .25, .5, Options{})
	if e != nil {
		t.Fatal(e)
	}
	old := r.Lower.Solid
	if !reflect.DeepEqual(old.Vertices(), r.Solid.Vertices()[:16]) || !reflect.DeepEqual(old.Edges(), r.Solid.Edges()[:24]) {
		t.Fatal("retained topology")
	}
	for i, f := range old.Faces() {
		if i != 1 && !reflect.DeepEqual(f, r.Solid.Faces()[i]) {
			t.Fatal("retained face", i)
		}
	}
	cones, planes := 0, 0
	for _, id := range r.BevelFaces {
		switch r.Solid.Faces()[id].Surface.(type) {
		case geom.ConeSurface:
			cones++
		case geom.PlaneSurface:
			planes++
		}
	}
	if cones != 4 || planes != 4 {
		t.Fatal("analytic bands")
	}
	mass, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Independently integrate three horizontal rounded-rectangle cross sections
	// by Simpson's rule, which is exact for their quadratic area polynomial.
	area := func(t float64) float64 { return (6-2*t)*(4-2*t) - (4-math.Pi)*(.75-t)*(.75-t) }
	volume := area(0)*2.5 + .5/6*(area(0)+4*area(.125)+area(.25))
	surfaceArea := area(0) + area(.25) + (14+2*math.Pi*.75)*2.5 + 14*math.Hypot(.25, .5) + math.Pi*(.75+.5)*math.Hypot(.25, .5)
	if math.Abs(mass.Area-surfaceArea) > 1e-7 {
		t.Fatal("independent planar/conical area", mass.Area, surfaceArea)
	}
	if math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-r.NominalSurfaceArea) > 1e-7 || math.Abs(r.NominalVolume-volume) > 1e-12 {
		t.Fatalf("mass %+v nominal %g %g independent %g", mass, r.NominalVolume, r.NominalSurfaceArea, volume)
	}
	var buf bytes.Buffer
	if e := trimmed.WriteJSON(&buf, r.Solid); e != nil {
		t.Fatal(e)
	}
	loaded, e := trimmed.ReadJSON(&buf, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := loaded.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .03, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	report, e := collision.SelfIntersectionsBVHContext(context.Background(), m, collision.Options{})
	if e != nil || !report.Complete || len(report.Contacts) != 0 {
		t.Fatal("display contacts", e, len(report.Contacts))
	}
}
func TestRoundedRectangleTopBevelReject(t *testing.T) {
	for _, v := range [][3]float64{{.75, .75, .5}, {.75, .25, 3}, {.75, -.25, .5}, {.75, .25, 0}, {.75, .25, .3}} {
		r, e := PrismRectangleTopBeveled(context.Background(), geom.Vec2{}, geom.Vec2{X: 6, Y: 4}, -1, 2, v[0], v[1], v[2], Options{})
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal("accepted", v)
		}
	}
}

func TestRoundedRectangleTopBevelCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := PrismRectangleTopBeveled(ctx, geom.Vec2{}, geom.Vec2{X: 6, Y: 4}, -1, 2, .75, .25, .5, Options{})
	if e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("cancel")
	}
}
