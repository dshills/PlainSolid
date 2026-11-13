package scene

import (
	"bytes"
	"context"
	"encoding/xml"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

func TestCameraProjectionRayAgreement(t *testing.T) {
	eye, target, up := geom.Vec3{X: 3, Y: -4, Z: 5}, geom.Vec3{X: 1, Z: 1}, geom.Vec3{Y: 1}
	for _, build := range []func() (Camera, error){
		func() (Camera, error) { return Orthographic(eye, target, up, 4, 3, .3, 20) },
		func() (Camera, error) { return Perspective(eye, target, up, math.Pi/3, 4.0/3, .3, 20) },
	} {
		c, e := build()
		if e != nil {
			t.Fatal(e)
		}
		for _, p := range []geom.Vec2{{}, {X: .5, Y: -.5}, {X: -.98, Y: .9}} {
			r, e := c.Ray(p)
			if e != nil {
				t.Fatal(e)
			}
			for _, k := range []float64{.01, .25, .95} {
				q, d, v, e := c.Project(r.Origin.Add(r.Direction.Scale(k * r.Length)))
				if e != nil || !v || math.Abs(q.X-p.X) > 1e-12 || math.Abs(q.Y-p.Y) > 1e-12 || math.Abs(d-(.3+19.7*k)) > 1e-12 {
					t.Fatalf("project %v to %v at %g visible %v: %v", p, q, d, v, e)
				}
			}
		}
	}
}

func TestPickClipAndInstances(t *testing.T) {
	c, e := Orthographic(geom.Vec3{Z: 10}, geom.Vec3{}, geom.Vec3{Y: 1}, 6, 6, 1, 8)
	if e != nil {
		t.Fatal(e)
	}
	m, e := mesh.Box(geom.Vec3{X: -1, Y: -1, Z: -1}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := New(map[string]mesh.Mesh{"cube": m}, []Item{
		{ID: "a", Asset: "cube", Transform: geom.Translation(geom.Vec3{Z: 5}).Mul(geom.Scaling(geom.Vec3{X: -2, Y: 1, Z: .5}))},
		{ID: "b", Asset: "cube", Transform: geom.Translation(geom.Vec3{Z: 3})},
		{ID: "clipped", Asset: "cube", Transform: geom.Translation(geom.Vec3{Z: 11})},
		{ID: "far", Asset: "cube", Transform: geom.Translation(geom.Vec3{X: 2.5, Z: -1})},
	})
	if e != nil {
		t.Fatal(e)
	}
	m.Vertices[0].Z = 100
	h, ok, e := s.Pick(c, geom.Vec2{X: .1, Y: .1})
	if e != nil || !ok || h.ID != "a" || math.Abs(h.Distance-3.5) > 1e-12 || math.Abs(h.Point.Z-5.5) > 1e-12 {
		t.Fatalf("hit %+v %v %v", h, ok, e)
	}
	if _, ok, e = s.Pick(c, geom.Vec2{X: .95, Y: .25}); e != nil || ok {
		t.Fatalf("unexpected far hit %v %v", ok, e)
	}
}

func TestHiddenLines(t *testing.T) {
	c, e := Orthographic(geom.Vec3{Z: 5}, geom.Vec3{}, geom.Vec3{Y: 1}, 4, 4, 1, 9)
	if e != nil {
		t.Fatal(e)
	}
	m := mesh.Mesh{
		Vertices:  []geom.Vec3{{X: -1, Y: -1, Z: 1}, {X: 1, Y: -1, Z: 1}, {X: 1, Y: 1, Z: 1}, {X: -1, Y: 1, Z: 1}},
		Triangles: [][3]int{{0, 1, 2}, {0, 2, 3}},
	}
	edges := []DrawingEdge{
		{ID: "behind", Start: geom.Vec3{X: -3}, End: geom.Vec3{X: 3}},
		{ID: "front", Start: geom.Vec3{X: -1, Z: 2}, End: geom.Vec3{X: 1, Z: 2}},
		{ID: "coplanar", Start: geom.Vec3{X: -1, Z: 1}, End: geom.Vec3{X: 1, Z: 1}},
		{ID: "out", Start: geom.Vec3{X: -1, Y: 3}, End: geom.Vec3{X: 1, Y: 3}},
	}
	r, e := HiddenLines(context.Background(), c, m, edges, HiddenLineOptions{DepthTolerance: 1e-12})
	if e != nil || len(r) != 4 {
		t.Fatalf("%+v %v", r, e)
	}
	for i, w := range [][2]float64{{-1, -.5}, {.5, 1}, {-.5, .5}, {-.5, .5}} {
		if math.Abs(r[i].Start.X-w[0]) > 1e-12 || math.Abs(r[i].End.X-w[1]) > 1e-12 {
			t.Fatalf("wrong visible span %+v", r[i])
		}
	}
	if _, e = HiddenLines(context.Background(), c, m, edges, HiddenLineOptions{MaxTests: 7}); e == nil {
		t.Fatal("work budget ignored")
	}
}

func TestPerspectiveHiddenLines(t *testing.T) {
	c, e := Perspective(geom.Vec3{}, geom.Vec3{Z: -1}, geom.Vec3{Y: 1}, math.Pi/2, 1, 1, 12)
	if e != nil {
		t.Fatal(e)
	}
	m := mesh.Mesh{Vertices: []geom.Vec3{{X: -1, Y: -1, Z: -2}, {X: 1, Y: -1, Z: -2}, {X: 1, Y: 1, Z: -2}, {X: -1, Y: 1, Z: -2}},
		Triangles: [][3]int{{0, 1, 2}, {0, 2, 3}}}
	edges := []DrawingEdge{{ID: "behind", Start: geom.Vec3{X: -3, Z: -4}, End: geom.Vec3{X: 6, Z: -8}}}
	x, e := HiddenLines(context.Background(), c, m, edges, HiddenLineOptions{})
	if e != nil || len(x) != 2 {
		t.Fatal(x, e)
	}
	for i, w := range [][4]float64{{-.75, -.5, 0, 1.0 / 11}, {.5, .75, 5.0 / 7, 1}} {
		h := x[i]
		if math.Abs(h.Start.X-w[0]) > 1e-12 || math.Abs(h.End.X-w[1]) > 1e-12 || math.Abs(h.T0-w[2]) > 1e-12 || math.Abs(h.T1-w[3]) > 1e-12 {
			t.Fatal(h, w)
		}
	}
	cut := []DrawingEdge{{ID: "passes near", Start: geom.Vec3{X: .1, Z: .5}, End: geom.Vec3{X: 1.5, Z: -3}}}
	x, e = HiddenLines(context.Background(), c, mesh.Mesh{}, cut, HiddenLineOptions{})
	if e != nil || len(x) != 1 || math.Abs(x[0].T0-3.0/7) > 1e-12 || x[0].T1 != 1 {
		t.Fatal(x, e)
	}
	// A constant-depth occluder clips the visible interval once edge depth
	// reaches triangle depth plus the requested world-space tolerance.
	edge := []DrawingEdge{{ID: "depth", Start: geom.Vec3{X: -.25, Z: -1.5}, End: geom.Vec3{X: .25, Z: -3.5}}}
	x, e = HiddenLines(context.Background(), c, m, edge, HiddenLineOptions{DepthTolerance: .25})
	if e != nil || len(x) != 1 || math.Abs(x[0].T1-.375) > 1e-12 {
		t.Fatal(x, e)
	}
}

func TestSVGDrawing(t *testing.T) {
	var b bytes.Buffer
	if e := WriteSVG(&b, []VisibleSpan{{ID: `<a&b>"`, Start: geom.Vec2{X: -1, Y: 1}, End: geom.Vec2{X: 1, Y: -1}}}, 320, 200); e != nil {
		t.Fatal(e)
	}
	var s struct {
		Lines []struct {
			ID string  `xml:"data-edge,attr"`
			X1 float64 `xml:"x1,attr"`
			Y1 float64 `xml:"y1,attr"`
			X2 float64 `xml:"x2,attr"`
			Y2 float64 `xml:"y2,attr"`
		} `xml:"g>line"`
	}
	if e := xml.Unmarshal(b.Bytes(), &s); e != nil {
		t.Fatal(e)
	}
	if len(s.Lines) != 1 || s.Lines[0].ID != `<a&b>"` || s.Lines[0].X1 != 0 || s.Lines[0].Y1 != 0 || s.Lines[0].X2 != 320 || s.Lines[0].Y2 != 200 {
		t.Fatal(s)
	}
}

func TestPlanarBoxDrawing(t *testing.T) {
	b, e := brep.Box(geom.Vec3{X: -1, Y: -1, Z: -1}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	camera, e := Orthographic(geom.Vec3{Z: 5}, geom.Vec3{}, geom.Vec3{Y: 1}, 4, 4, 1, 9)
	if e != nil {
		t.Fatal(e)
	}
	edges := PlanarEdges(b)
	if len(edges) != 12 {
		t.Fatalf("got %d box edges", len(edges))
	}
	s, e := HiddenLines(context.Background(), camera, b.Tessellate(), edges, HiddenLineOptions{DepthTolerance: 1e-10})
	if e != nil || len(s) != 4 {
		t.Fatalf("%+v %v", s, e)
	}
	for _, x := range s {
		i, e := ParseEdgeID(x.ID, b)
		if e != nil || edges[i].Start.Z != 1 || edges[i].End.Z != 1 {
			t.Fatalf("unexpected visible edge %q", x.ID)
		}
	}
}

func TestCurvedDrawingEdges(t *testing.T) {
	f, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := parametric.Cylinder(f, 2, 3)
	if e != nil {
		t.Fatal(e)
	}
	edges, e := ParametricEdges(s, geom.CurveTessellationOptions{ChordTolerance: .01}, 10000)
	if e != nil {
		t.Fatal(e)
	}
	length := 0.
	for _, c := range edges {
		if c.ID == "0" {
			length += c.End.Sub(c.Start).Length()
		}
	}
	if math.Abs(length-4*math.Pi) > .025 {
		t.Fatalf("circular rim chord length %g", length)
	}
	if _, e = ParametricEdges(s, geom.CurveTessellationOptions{ChordTolerance: .01}, 1); e == nil {
		t.Fatal("edge budget ignored")
	}
}
