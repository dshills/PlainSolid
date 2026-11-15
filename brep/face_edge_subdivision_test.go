package brep

import (
	"math"
	"strings"
	"testing"

	"plainsolid/geom"
)

func splitOriginalFaceEdge(t *testing.T, s Solid, id int, stations ...float64) (Solid, error) {
	t.Helper()
	v, e, f := s.Vertices(), s.Edges(), s.Faces()
	old := e[id]
	from, to := v[old.Start], v[old.End]
	ids := []int{id}
	last := old.Start
	for i, t := range stations {
		j := len(v)
		v = append(v, from.Add(to.Sub(from).Scale(t)))
		if i == 0 {
			e[id] = Edge{Start: last, End: j}
		} else {
			ids = append(ids, len(e))
			e = append(e, Edge{Start: last, End: j})
		}
		last = j
	}
	ids = append(ids, len(e))
	e = append(e, Edge{Start: last, End: old.End})
	rewrite := func(w []EdgeUse) []EdgeUse {
		out := make([]EdgeUse, 0, len(w)+len(stations))
		for _, u := range w {
			if u.Edge != id {
				out = append(out, u)
				continue
			}
			for i, e := range ids {
				if u.Reversed {
					e = ids[len(ids)-1-i]
				}
				out = append(out, EdgeUse{Edge: e, Reversed: u.Reversed})
			}
		}
		return out
	}
	for i := range f {
		f[i].Boundary = rewrite(f[i].Boundary)
		for j := range f[i].Holes {
			f[i].Holes[j] = rewrite(f[i].Holes[j])
		}
	}
	return NewSolid(v, e, f)
}

func TestPlanarBRepKeepsCollinearOriginalEdgeStations(t *testing.T) {
	box, err := Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	s, err := splitOriginalFaceEdge(t, box, 0, .25, .75)
	if err != nil {
		t.Fatal(err)
	}
	mesh := s.Tessellate()
	if len(s.Vertices()) != 10 || len(s.Edges()) != 14 || len(mesh.Triangles) != 16 || len(s.TriangleFaces()) != 16 {
		t.Fatalf("original edge stations lost: %+v", s.Topology())
	}
	p, err := s.MassProperties()
	if err != nil || math.Abs(p.Volume-1) > 1e-13 || math.Abs(p.SurfaceArea-6) > 1e-13 {
		t.Fatalf("edge split changed represented body: %+v %v", p, err)
	}
	for _, i := range []int{0, 12, 13} {
		e := s.Edges()[i]
		uses := 0
		for _, f := range mesh.Triangles {
			for j, a := range f {
				b := f[(j+1)%3]
				if a == e.Start && b == e.End || b == e.Start && a == e.End {
					uses++
				}
			}
		}
		if uses != 2 {
			t.Fatalf("missing original mesh coedges for edge %d: %d", i, uses)
		}
	}
	if _, err := splitOriginalFaceEdge(t, box, 0, 1.25); err == nil || !strings.Contains(err.Error(), "retraces") {
		t.Fatalf("a collinear reversed use was accepted: %v", err)
	}
}

func TestPlanarBRepKeepsOriginalStationsOnAHole(t *testing.T) {
	square := func(a, b float64) []geom.Vec3 {
		return []geom.Vec3{{X: a, Y: a}, {X: b, Y: a}, {X: b, Y: b}, {X: a, Y: b}}
	}
	a, err := PrismWithHoles(square(0, 4), [][]geom.Vec3{square(1, 3)}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	src := a.Faces()[0].Holes[0][0].Edge
	b, err := splitOriginalFaceEdge(t, a, src, .25, .5, .75)
	if err != nil {
		t.Fatal(err)
	}
	old, err := a.MassProperties()
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.MassProperties()
	if err != nil || math.Abs(got.Volume-old.Volume) > 1e-13 || math.Abs(got.SurfaceArea-old.SurfaceArea) > 1e-13 ||
		len(b.Faces()[0].Holes[0]) != 7 || len(b.Tessellate().Triangles) != len(a.Tessellate().Triangles)+6 {
		t.Fatalf("hole edge stations changed original material: %+v %+v %v", old, got, err)
	}
}
