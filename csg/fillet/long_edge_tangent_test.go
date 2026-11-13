package fillet

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestLongEdgeTangentPocket(t *testing.T) {
	for _, back := range []bool{false, true} {
		for _, depth := range []float64{10, 15} {
			var corners [4]LongEdgeCorner
			p := trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 10, Y: 2}, Max: geom.Vec2{X: 13, Y: 5}}
			corners[3].Radius = 2
			corners[0] = LongEdgeCorner{YSetback: 1, ZSetback: 1}
			if back {
				corners = [4]LongEdgeCorner{}
				corners[2].Radius = 2
				corners[1] = LongEdgeCorner{YSetback: 1, ZSetback: 1}
				p.Min.Y, p.Max.Y = 5, 8
			}
			b := trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 5}, Radius: 1}

			r, e := TreatLongEdgesGrouped(context.Background(), 20, 10, depth, corners, &b, &p, nil, Options{})
			if e != nil {
				t.Fatal(back, e)
			}
			want := 200*depth - 80*(1-math.Pi/4) - 10 - depth*math.Pi - 9*depth
			m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
			if e != nil || math.Abs(m.Volume-want) > 1e-7 || math.Abs(m.Area-r.NominalSurfaceArea) > 1e-7 {
				t.Fatal(back, e, m, want)
			}
			mesh, e := r.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
			if e != nil {
				t.Fatal(back, e)
			}
			if e = mesh.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			if len(r.Solid.Faces()[r.Stock.TopFace].Holes) != 1 || len(r.Solid.Faces()[r.Stock.BottomFace].Holes) != 2 {
				t.Fatal("notch must merge outer boundary")
			}
			var data bytes.Buffer
			if e = trimmed.WriteJSON(&data, r.Solid); e != nil {
				t.Fatal(e)
			}
			loaded, e := trimmed.ReadJSON(&data, trimmed.JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			lm, e := loaded.MassProperties(context.Background(), parametric.IntegrationOptions{})
			if e != nil || math.Abs(lm.Volume-want) > 1e-7 {
				t.Fatal("persistence", e, lm)
			}
		}
	}
}

func TestLongEdgeTangentPocketRejectsCrossing(t *testing.T) {
	corners := [4]LongEdgeCorner{{YSetback: 1, ZSetback: 1}, {}, {}, {Radius: 2}}
	for _, y := range []float64{math.Nextafter(2, 0), math.Nextafter(2, 3), 1} {
		p := trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 10, Y: y}, Max: geom.Vec2{X: 13, Y: 5}}
		if _, e := TreatLongEdgesGrouped(context.Background(), 20, 10, 15, corners, nil, &p, nil, Options{}); e == nil {
			t.Fatal("unqualified crossing/nearcontact", y)
		}
	}
}

func TestLongEdgeTangentPocketWithBoss(t *testing.T) {
	corners := [4]LongEdgeCorner{{}, {YSetback: 1, ZSetback: 1}, {Radius: 2}, {}}
	p := trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 10, Y: 5}, Max: geom.Vec2{X: 13, Y: 8}}
	b := trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 5}, Radius: 1}
	boss := trimmed.ExtrusionBoss{Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 15, Y: 5}, Max: geom.Vec2{X: 17, Y: 7}}, Height: 3, Upper: true}
	r, e := TreatLongEdgesGrouped(context.Background(), 20, 10, 15, corners, &b, &p, &boss, Options{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	want := 3000 - 80*(1-math.Pi/4) - 10 - 15*math.Pi - 135 + 12
	if e != nil || math.Abs(m.Volume-want) > 1e-7 || math.Abs(m.Area-r.NominalSurfaceArea) > 1e-7 {
		t.Fatal(e, m, want)
	}
	mesh, e := r.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
}
