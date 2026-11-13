package fillet

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestSelectedRectangleBore(t *testing.T) {
	ctx := context.Background()
	r, e := PrismRectangleCornersBored(ctx, geom.Vec2{}, geom.Vec2{X: 4, Y: 3}, -1, 1, .25, []CornerRadius{{Corner: 0, Radius: .25}, {Corner: 2, Radius: .5}}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	oldV, oldE, oldF := r.Stock.Solid.Vertices(), r.Stock.Solid.Edges(), r.Stock.Solid.Faces()
	if !reflect.DeepEqual(oldV, r.Solid.Vertices()[:len(oldV)]) || !reflect.DeepEqual(oldE, r.Solid.Edges()[:len(oldE)]) {
		t.Fatal("changed original topology")
	}
	faces := r.Solid.Faces()
	for i, f := range oldF {
		if i == r.Stock.Caps[0] || i == r.Stock.Caps[1] {
			if len(faces[i].Holes) != 1 {
				t.Fatal("cap hole")
			}
			faces[i].Holes = f.Holes
		}
		if !reflect.DeepEqual(f, faces[i]) {
			t.Fatalf("changed face %d", i)
		}
	}
	if len(r.Solid.Vertices()) != len(oldV)+8 || len(r.Solid.Edges()) != len(oldE)+12 || len(faces) != len(oldF)+4 {
		t.Fatal("bore ancestry counts")
	}
	props, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	area := 12 - (1-math.Pi/4)*(.25*.25+.5*.5) - math.Pi*.25*.25
	perimeter := 14 - 2*(.25+.5) + math.Pi/2*(.25+.5) + 2*math.Pi*.25
	if math.Abs(props.Volume-2*area) > 1e-7 || math.Abs(props.Area-(2*area+2*perimeter)) > 1e-7 {
		t.Fatalf("mass %+v", props)
	}
	m, e := r.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	report, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if e != nil || !report.Complete || len(report.Contacts) != 0 {
		t.Fatalf("display contacts: %v %+v", e, report)
	}
	edges := map[[2]int]bool{}
	for _, tri := range m.Triangles {
		for j, a := range tri {
			b := tri[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}] = true
		}
	}
	if len(m.Vertices)-len(edges)+len(m.Triangles) != 0 {
		t.Fatal("expected through-bore genus one")
	}
}

func TestSelectedRectangleBoreRejects(t *testing.T) {
	for _, r := range []float64{0, -1, .5, 1, math.NaN()} {
		out, e := PrismRectangleCornersBored(context.Background(), geom.Vec2{}, geom.Vec2{X: 4, Y: 3}, 0, 2, r, []CornerRadius{{Corner: 0, Radius: .5}}, Options{})
		if e == nil || len(out.Solid.Faces()) != 0 {
			t.Fatalf("accepted radius %v", r)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := PrismRectangleCornersBored(ctx, geom.Vec2{}, geom.Vec2{X: 4, Y: 3}, 0, 2, .25, []CornerRadius{{Corner: 0, Radius: .5}}, Options{}); e == nil {
		t.Fatal("cancel")
	}
}
