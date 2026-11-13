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

func TestBoredRoundedRectangleBevel(t *testing.T) {
	ctx := context.Background()
	r, e := PrismRectangleTopBeveledBored(ctx, geom.Vec2{}, geom.Vec2{X: 6, Y: 4}, -1, 2, .75, .25, .5, .25, Options{})
	if e != nil {
		t.Fatal(e)
	}
	before := r.Stock.Solid
	if !reflect.DeepEqual(before.Vertices(), r.Solid.Vertices()[:len(before.Vertices())]) || !reflect.DeepEqual(before.Edges(), r.Solid.Edges()[:len(before.Edges())]) {
		t.Fatal("prior topology changed")
	}
	fs := r.Solid.Faces()
	for i, f := range before.Faces() {
		got := fs[i]
		if i == 0 || i == 1 {
			if len(got.Holes) != 1 {
				t.Fatal("cap bore")
			}
			got.Holes = f.Holes
		}
		if !reflect.DeepEqual(f, got) {
			t.Fatal("prior face changed", i)
		}
	}
	if len(r.Solid.Vertices()) != len(before.Vertices())+8 || len(r.Solid.Edges()) != len(before.Edges())+12 || len(fs) != len(before.Faces())+4 {
		t.Fatal("bore ancestry counts")
	}
	props, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	old, e := before.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	volumeDelta := math.Pi * .25 * .25 * 3
	areaDelta := 2*math.Pi*.25*3 - 2*math.Pi*.25*.25
	if math.Abs(old.Volume-props.Volume-volumeDelta) > 1e-7 || math.Abs(props.Area-old.Area-areaDelta) > 1e-7 || math.Abs(props.Volume-r.NominalVolume) > 1e-7 || math.Abs(props.Area-r.NominalSurfaceArea) > 1e-7 {
		t.Fatal("analytic differential", old, props)
	}
	var encoded bytes.Buffer
	if e := trimmed.WriteJSON(&encoded, r.Solid); e != nil {
		t.Fatal(e)
	}
	loaded, e := trimmed.ReadJSON(&encoded, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := loaded.Tessellate(ctx, parametric.TessellationOptions{Deflection: .03, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	report, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if e != nil || !report.Complete || len(report.Contacts) != 0 {
		t.Fatal("contacts", e, len(report.Contacts))
	}
	edges := map[[2]int]bool{}
	for _, tr := range m.Triangles {
		for j, a := range tr {
			b := tr[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}] = true
		}
	}
	if len(m.Vertices)-len(edges)+len(m.Triangles) != 0 {
		t.Fatal("genus")
	}
}
func TestBoredRoundedRectangleBevelClearance(t *testing.T) {
	for _, radius := range []float64{0, -1, .5, 1, math.NaN()} {
		r, e := PrismRectangleTopBeveledBored(context.Background(), geom.Vec2{}, geom.Vec2{X: 6, Y: 4}, -1, 2, .75, .25, .5, radius, Options{})
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal("accepted radius", radius)
		}
	}
	// Radius at the exact central-box limit must fail even though the ideal
	// circle still lies inside the less conservative physical profile.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := PrismRectangleTopBeveledBored(ctx, geom.Vec2{}, geom.Vec2{X: 6, Y: 4}, -1, 2, .75, .25, .5, .25, Options{})
	if e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("cancel")
	}
}
