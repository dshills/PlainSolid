package draft

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func taperedBoxEdge(t *testing.T, s brep.Solid, x, y float64) (int, int) {
	t.Helper()
	v := s.Vertices()
	edge := -1
	for i, e := range s.Edges() {
		if v[e.Start].X == x && v[e.End].X == x && v[e.Start].Y == y && v[e.End].Y == y {
			edge = i
		}
	}
	if edge < 0 {
		t.Fatal("fixture edge")
	}
	for i, f := range s.Faces() {
		for _, u := range f.Boundary {
			if u.Edge == edge {
				return edge, i
			}
		}
	}
	t.Fatal("fixture face")
	return 0, 0
}
func TestTaperedChamferIndependentIntegral(t *testing.T) {
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 3, Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	edge, face := taperedBoxEdge(t, s, 0, 0)
	other, otherFace := taperedBoxEdge(t, s, 4, 3)
	a := TaperedEdgeChamfer{edge, face, .25, .5, .5, 1}
	b := TaperedEdgeChamfer{other, otherFace, .125, .25, .25, .5}
	for _, cs := range [][]TaperedEdgeChamfer{{a}, {a, b}, {a, {other, otherFace, .25, .5, .25, .5}}} {
		r, e := ChamferTapered(context.Background(), s, cs, false, Options{})
		if e != nil {
			t.Fatal(e)
		}
		removed := big.NewRat(7, 24)
		if len(cs) == 2 {
			if cs[1].StartSetback == cs[1].EndSetback {
				removed.Add(removed, big.NewRat(1, 8))
			} else {
				removed.Add(removed, big.NewRat(7, 96))
			}
		}
		want := new(big.Rat).Sub(big.NewRat(24, 1), removed)
		if r.ExactVolume.Cmp(want) != 0 {
			t.Fatal("integrated triangular cross sections", r.ExactVolume, want)
		}
		for i, proof := range r.Sections {
			if proof.Selection != cs[i] || proof.Vertices != [2]int{s.Edges()[cs[i].Edge].Start, s.Edges()[cs[i].Edge].End} {
				t.Fatal("lost original width direction")
			}
			id := r.EdgeFaces[i]
			if id < 0 || len(r.FaceEdges[id]) != 1 || r.FaceEdges[id][0] != cs[i].Edge {
				t.Fatal("bevel ancestry")
			}
			h := r.FacePlanes[id]
			for _, p := range proof.OffsetPoints {
				if dot(h.Normal, p).Cmp(h.Offset) != 0 {
					t.Fatal("endpoint off exact bevel")
				}
			}
			for _, alpha := range []*big.Rat{big.NewRat(1, 7), big.NewRat(3, 5)} {
				for side := 0; side < 2; side++ {
					p := addPoint(scale(proof.OffsetPoints[side], new(big.Rat).Sub(big.NewRat(1, 1), alpha)), scale(proof.OffsetPoints[side+2], alpha))
					if dot(h.Normal, p).Cmp(h.Offset) != 0 {
						t.Fatal("continuous width field")
					}
				}
			}
		}
		mesh := r.Solid.Tessellate()
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		hits, e := collision.SelfIntersectionsBVHContext(context.Background(), mesh, collision.Options{})
		if e != nil || !hits.Complete || len(hits.Contacts) > 0 {
			t.Fatal("display", e)
		}
	}
}
func addPoint(a, b point) point { return point{add(a[0], b[0]), add(a[1], b[1]), add(a[2], b[2])} }
func TestTaperedChamferConstantAndReversal(t *testing.T) {
	s := cube(t)
	id := cornerEdge(t, s)
	c := TaperedEdgeChamfer{id, 3, .25, .5, .25, .5}
	a, e := ChamferTapered(context.Background(), s, []TaperedEdgeChamfer{c}, false, Options{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := Chamfer(context.Background(), s, []EdgeChamfer{{id, 3, .25, .5}}, false, Options{})
	if e != nil || !reflect.DeepEqual(a.Result, b.Result) {
		t.Fatal("constant width compatibility", e)
	}
	c.EndSetback, c.EndOtherSetback = .5, 1
	a, e = ChamferTapered(context.Background(), s, []TaperedEdgeChamfer{c}, false, Options{})
	if e != nil {
		t.Fatal(e)
	}
	edges, faces := s.Edges(), s.Faces()
	edges[id].Start, edges[id].End = edges[id].End, edges[id].Start
	for i := range faces {
		for j := range faces[i].Boundary {
			if faces[i].Boundary[j].Edge == id {
				faces[i].Boundary[j].Reversed = !faces[i].Boundary[j].Reversed
			}
		}
	}
	reversed, e := brep.NewSolid(s.Vertices(), edges, faces)
	if e != nil {
		t.Fatal(e)
	}
	c.StartSetback, c.EndSetback = c.EndSetback, c.StartSetback
	c.StartOtherSetback, c.EndOtherSetback = c.EndOtherSetback, c.StartOtherSetback
	r, e := ChamferTapered(context.Background(), reversed, []TaperedEdgeChamfer{c}, false, Options{})
	if e != nil || r.ExactVolume.Cmp(a.ExactVolume) != 0 || !reflect.DeepEqual(r.ExactVertices, a.ExactVertices) {
		t.Fatal("original edge reversed", e)
	}
}
func TestTaperedChamferAtomicFailures(t *testing.T) {
	s := cube(t)
	id := cornerEdge(t, s)
	valid := TaperedEdgeChamfer{id, 3, .125, .25, .25, .5}
	bad := valid
	bad.EndOtherSetback = .75
	for _, cs := range [][]TaperedEdgeChamfer{{bad}, {valid, bad}, {valid, valid}, {{id, 0, .125, .25, .25, .5}}, {{id, 3, math.NaN(), .25, .25, .5}}, {{id, 3, 0, .25, .25, .5}}} {
		r, e := ChamferTapered(context.Background(), s, cs, false, Options{})
		if e == nil || len(r.Solid.Faces()) != 0 || len(r.Sections) != 0 {
			t.Fatal("partial invalid selection", e)
		}
	}
	if _, e := ChamferTapered(context.Background(), s, []TaperedEdgeChamfer{bad}, false, Options{}); !errors.Is(e, ErrUnsupported) {
		t.Fatal("noncoplanar", e)
	}
	if _, e := ChamferTapered(context.Background(), s, []TaperedEdgeChamfer{valid}, false, Options{MaxWork: 1}); !errors.Is(e, ErrLimit) {
		t.Fatal("budget", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := ChamferTapered(ctx, s, []TaperedEdgeChamfer{valid}, false, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestTaperedChamferCoordinateMetric(t *testing.T) {
	s, e := brep.Prism([]geom.Vec3{{}, {X: 1}, {X: 1, Y: 1}}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	id, face := taperedBoxEdge(t, s, 0, 0)
	c := TaperedEdgeChamfer{id, face, .125, .25, .25, .5}
	if _, e = ChamferTapered(context.Background(), s, []TaperedEdgeChamfer{c}, false, Options{}); !errors.Is(e, ErrUnsupported) {
		t.Fatal("irrational Euclidean direction", e)
	}
	r, e := ChamferTapered(context.Background(), s, []TaperedEdgeChamfer{c}, true, Options{})
	if e != nil {
		t.Fatal(e)
	}
	// The coordinate-setback corner triangle has area ab/2; both widths
	// double linearly over unit height, giving removed volume 7/192.
	if r.ExactVolume.Cmp(big.NewRat(89, 192)) != 0 {
		t.Fatal("coordinate integral", r.ExactVolume)
	}
}
