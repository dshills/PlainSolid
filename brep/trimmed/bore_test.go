package trimmed

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBoreCylinder(t *testing.T) {
	b, e := brep.Box(geom.Vec3{X: -2, Y: -2, Z: -1}, geom.Vec3{X: 4, Y: 3, Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	center := geom.Vec3{X: .5, Y: .25, Z: 8}
	const r = .75
	for _, order := range [][2]int{{0, 1}, {1, 0}} {
		s, h, e := BoreCylinder(b, order[0], order[1], center, r, 1e-10)
		if e != nil {
			t.Fatal(e)
		}
		if h.Entry != order[0] || h.Exit != order[1] || h.EntryEdges != [4]int{12, 13, 14, 15} ||
			h.ExitEdges != [4]int{16, 17, 18, 19} || h.WallFaces != [4]int{6, 7, 8, 9} ||
			len(s.Faces()[h.Entry].Holes) != 1 || len(s.Faces()[h.Exit].Holes) != 1 {
			t.Fatal(h)
		}
		for _, f := range h.WallFaces {
			if _, ok := s.Faces()[f].Surface.(geom.CylinderSurface); !ok {
				t.Fatal("wall lost its cylinder")
			}
		}
		var buf bytes.Buffer
		if e = WriteJSON(&buf, s); e != nil {
			t.Fatal(e)
		}
		s, e = ReadJSON(&buf, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		removed := 3 * math.Pi * r * r
		volume := 90 - removed
		area := 126 - 2*math.Pi*r*r + 6*math.Pi*r
		centroid := geom.Vec3{X: (90 - removed*.5) / volume, Y: (45 - removed*.25) / volume, Z: .5}
		if e != nil || math.Abs(p.Volume-volume) > 1e-8 || math.Abs(p.Area-area) > 1e-8 || p.Centroid.Sub(centroid).Length() > 1e-8 {
			t.Fatalf("properties=%+v err=%v, want V=%v A=%v center=%v", p, e, volume, area, centroid)
		}
		m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
}

func TestBoreCylinderRejectsContact(t *testing.T) {
	b, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 5, Z: 6})
	for _, tc := range []struct {
		a, b int
		p    geom.Vec3
		r    float64
	}{
		{0, 1, geom.Vec3{X: 1, Y: 2}, 1},
		{0, 1, geom.Vec3{X: 1, Y: 2}, 1.1},
		{0, 2, geom.Vec3{X: 2, Y: 2}, .25},
		{0, 1, geom.Vec3{X: 2, Y: 2}, math.Inf(1)},
	} {
		if _, _, e := BoreCylinder(b, tc.a, tc.b, tc.p, tc.r, 1e-10); e == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	concave, e := brep.Prism([]geom.Vec3{{}, {X: 4}, {X: 4, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 4}, {Y: 4}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = BoreCylinder(concave, 0, 1, geom.Vec3{X: .5, Y: .5}, .2, 1e-10); e == nil {
		t.Fatal("accepted nonconvex input")
	}
}

func TestBoreCylinderOblique(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 5, Z: 6})
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	const r = .35
	for _, tc := range []struct {
		name      string
		exit      int
		d, start  geom.Vec3
		h, da, db float64
	}{
		{"parallel caps", 1, geom.Vec3{X: 1, Z: 3}, geom.Vec3{X: 1, Y: 2}, 2 * math.Sqrt(10), 3 / math.Sqrt(10), 3 / math.Sqrt(10)},
		{"adjacent faces", 3, geom.Vec3{X: 1, Z: 1}, geom.Vec3{X: 1, Y: 2}, 3 * math.Sqrt2, 1 / math.Sqrt2, 1 / math.Sqrt2},
		{"different cap angles", 3, geom.Vec3{X: 2, Z: 1}, geom.Vec3{X: 1, Y: 2}, 1.5 * math.Sqrt(5), 1 / math.Sqrt(5), 2 / math.Sqrt(5)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, h, e := BoreCylinderAlong(b, 0, tc.exit, tc.start, tc.d, r, 1e-10)
			if e != nil {
				t.Fatal(e)
			}
			face := s.Faces()[h.Exit]
			arc := face.Holes[0][0].Curve.(EllipseUV)
			uv, e := arc.EvalUV(.52)
			if e != nil {
				t.Fatal(e)
			}
			p, e := face.Surface.Eval(uv.X, uv.Y)
			if e != nil {
				t.Fatal(e)
			}
			n, _ := tc.d.Unit()
			if d := p.Sub(tc.start).Cross(n).Length(); math.Abs(d-r) > 1e-11 {
				t.Fatalf("rim radius %v, want %v", d, r)
			}
			var buf bytes.Buffer
			if e = WriteJSON(&buf, s); e != nil {
				t.Fatal(e)
			}
			s, e = ReadJSON(&buf, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			props, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
			volume := 120 - math.Pi*r*r*tc.h
			area := 148 + 2*math.Pi*r*tc.h - math.Pi*r*r*(1/tc.da+1/tc.db)
			if e != nil || math.Abs(props.Volume-volume) > 1e-8 || math.Abs(props.Area-area) > 1e-8 {
				t.Fatalf("properties=%+v err=%v, want V=%v A=%v", props, e, volume, area)
			}
			m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestMultipleThroughBores(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 8, Y: 6, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	cuts := []CylinderCut{
		{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 2, Y: 3}, Direction: geom.Vec3{Z: 1}, Radius: .5},
		{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 6, Y: 3}, Direction: geom.Vec3{Z: 1}, Radius: .75},
	}
	s, h, e := BoreCylinders(b, cuts, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	if len(h) != 2 || h[1].WallFaces[0] != 10 || h[0].EntryEdges[0] == h[1].EntryEdges[0] ||
		len(s.Faces()[0].Holes) != 2 || len(s.Faces()[1].Holes) != 2 {
		t.Fatal(h)
	}
	ctx := context.Background()
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	v := 192 - 4*math.Pi*(.5*.5+.75*.75)
	a := 208 + 8*math.Pi*(.5+.75) - 2*math.Pi*(.5*.5+.75*.75)
	if e != nil || math.Abs(p.Volume-v) > 1e-8 || math.Abs(p.Area-a) > 1e-8 {
		t.Fatal(p, v, a, e)
	}
	m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .025})
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal(e)
	}
	box, e := s.Bounds()
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range m.Vertices {
		if !box.Contains(p) {
			t.Fatal(box, p)
		}
	}
	cuts[1].AxisPoint.X = 2.5
	if _, _, e = BoreCylinders(b, cuts, 1e-10); e == nil {
		t.Fatal("accepted overlapping bores")
	}
}
