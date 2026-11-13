package fillet

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestGroupedLongEdgesAllCornerAssignments(t *testing.T) {
	for code := 0; code < 81; code++ {
		k := code
		var c [4]LongEdgeCorner
		area, perimeter := 100., 40.
		for i := range c {
			switch k % 3 {
			case 1:
				c[i].Radius = 1
				area -= 1 - math.Pi/4
				perimeter += math.Pi/2 - 2
			case 2:
				c[i].YSetback = 1
				c[i].ZSetback = 2
				area -= 1
				perimeter += math.Sqrt(5) - 3
			}
			k /= 3
		}
		r, e := TreatLongEdgesGrouped(context.Background(), 20, 10, 10, c, nil, nil, nil, Options{})
		if e != nil {
			t.Fatal(code, e)
		}
		m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-20*area) > 1e-7 || math.Abs(m.Area-(2*area+20*perimeter)) > 1e-7 {
			t.Fatal(code, e, m, area, perimeter)
		}
		mesh, e := r.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
		if e != nil {
			t.Fatal(code, e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(code, e)
		}
		for i, x := range c {
			if (x.Radius > 0) != (r.Stock.CornerFilletFaces[i] >= 0) || (x.YSetback > 0) != (r.Stock.CornerChamferFaces[i] >= 0) {
				t.Fatal("corner ancestry", code, i)
			}
		}
	}
}
func TestGroupedLongEdgesRejectOverlap(t *testing.T) {
	for _, c := range [][4]LongEdgeCorner{{{Radius: 6}, {Radius: 4}}, {{YSetback: 3, ZSetback: 6}, {}, {}, {Radius: 4}}, {{Radius: 1, YSetback: 1, ZSetback: 1}}, {{YSetback: 1}}, {{Radius: math.NaN()}}} {
		if _, e := TreatLongEdgesGrouped(context.Background(), 20, 10, 10, c, nil, nil, nil, Options{}); e == nil {
			t.Fatal("accepted", c)
		}
	}
}
func TestGroupedLongEdgesPhysicalCorners(t *testing.T) {
	c := [4]LongEdgeCorner{{YSetback: 1, ZSetback: 1}, {Radius: 2}, {}, {Radius: 1}}
	r, e := TreatLongEdgesGrouped(context.Background(), 20, 10, 10, c, nil, nil, nil, Options{})
	if e != nil {
		t.Fatal(e)
	}
	for corner, id := range r.Stock.CornerFilletFaces {
		if id < 0 {
			continue
		}
		f := r.Solid.Faces()[id]
		p, e := f.Surface.Eval((f.Outer[0].UVStart.X+f.Outer[2].UVStart.X)/2, (f.Outer[0].UVStart.Y+f.Outer[2].UVStart.Y)/2)
		if e != nil {
			t.Fatal(e)
		}
		if corner == 1 && (p.Y < 8 || p.Z > 2) || corner == 3 && (p.Y > 1 || p.Z < 9) {
			t.Fatal("wrong physical corner", corner, p)
		}
	}
	// Rectangular cap query metadata independently names actual flat intervals.
	if r.Stock.TopMinY != 1 || r.Stock.TopMaxY != 10 || r.Stock.BottomMinY != 1 || r.Stock.BottomMaxY != 8 {
		t.Fatal("cap strips", r.Stock)
	}
}

func TestGroupedLongEdgesWithFeatures(t *testing.T) {
	c := [4]LongEdgeCorner{{YSetback: 1, ZSetback: 1}, {Radius: 1}, {Radius: 1}, {Radius: 1}}
	bore := trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 5}, Radius: 1}
	pocket := trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 10, Y: 4}, Max: geom.Vec2{X: 13, Y: 6}}
	boss := trimmed.ExtrusionBoss{Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 15, Y: 4}, Max: geom.Vec2{X: 17, Y: 6}}, Height: 3, Upper: true}
	r, e := TreatLongEdgesGrouped(context.Background(), 20, 10, 10, c, &bore, &pocket, &boss, Options{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-r.NominalVolume) > 1e-7 || math.Abs(m.Area-r.NominalSurfaceArea) > 1e-7 {
		t.Fatal(e, m)
	}
	mesh, e := r.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
}
