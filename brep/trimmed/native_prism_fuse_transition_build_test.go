package trimmed_test

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestNativeCrossingPrismFuseClosed(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	b := transitionPrism(t, geom.Vec3{X: 1, Y: .5, Z: 1}, geom.Vec3{X: 3, Y: 1.5, Z: 4})
	for _, swap := range []bool{false, true} {
		owners := [2]brep.Solid{a, b}
		if swap {
			owners[0], owners[1] = b, a
		}
		proof, e := trimmed.ImprintCrossingNativePrismFuse(context.Background(), owners[0], owners[1], trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		q, e := proof.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{})
		if e != nil {
			t.Fatal(e)
		}
		s := q.Solid()
		if e = s.Validate(); e != nil {
			t.Fatal(e)
		}
		m, e := s.MassProperties()
		if e != nil || math.Abs(m.Volume-16) > 1e-11 || math.Abs(m.Centroid.X-25./16) > 1e-12 || math.Abs(m.Centroid.Y-1) > 1e-12 || math.Abs(m.Centroid.Z-29./16) > 1e-12 {
			t.Fatal(m, e)
		}
		if len(s.Vertices())-len(s.Edges())+len(s.Faces()) != 2 {
			t.Fatal("incorrect closed boundary")
		}
		if q.Transition() != proof || len(q.Faces()) != len(s.Faces()) {
			t.Fatal("lost original certificate")
		}
		capOwners := [2]bool{}
		unionWalls := 0
		for _, u := range q.Faces() {
			if u.Reversed {
				t.Fatal("reversed Fuse support")
			}
			if u.Region == 2 {
				unionWalls++
			}
			if u.Region >= 0 && u.Region < 2 && u.Edge == -1 && u.Cycle >= 0 {
				capOwners[u.Operand] = true
			}
			normal := func(s brep.Solid, f brep.Face) (geom.Vec3, geom.Vec3) {
				v, edges := s.Vertices(), s.Edges()
				points := []geom.Vec3{}
				for _, u := range f.Boundary {
					e := edges[u.Edge]
					id := e.Start
					if u.Reversed {
						id = e.End
					}
					points = append(points, v[id])
				}
				return points[0], points[1].Sub(points[0]).Cross(points[2].Sub(points[0]))
			}
			p, n := normal(owners[u.Operand], owners[u.Operand].Faces()[u.OriginalFace])
			_, got := normal(s, s.Faces()[u.Face])
			if n.Dot(got) <= 0 {
				t.Fatal("wrong source normal", u)
			}
			for _, edge := range s.Faces()[u.Face].Boundary {
				v := s.Vertices()[s.Edges()[edge.Edge].Start]
				if n.Dot(v.Sub(p)) != 0 {
					t.Fatal("off original support", u)
				}
			}
		}
		if !capOwners[0] || !capOwners[1] || unionWalls != len(proof.UnionProfile().Edges()) {
			t.Fatal("missing shoulder/overlap wall")
		}
		for _, u := range q.WallUses() {
			if q.Faces()[u.Use.Face].OriginalFace != u.Use.Wall || q.Faces()[u.Use.Face].Operand != u.Use.Operand {
				t.Fatal("wrong original wall")
			}
			for _, h := range u.HeightParameter {
				if h.Sign() < 0 || h.Cmp(big.NewRat(1, 1)) > 0 {
					t.Fatal(h)
				}
			}
		}
		rebuilt, e := brep.NewSolid(s.Vertices(), s.Edges(), s.Faces())
		if e != nil {
			t.Fatal(e)
		}
		rm, _ := rebuilt.MassProperties()
		if rm.Volume != m.Volume {
			t.Fatal(rm)
		}
		for _, limit := range []int{q.NativeOperations(), q.NativeOperations() - 1} {
			out, e := proof.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{MaxOperations: limit})
			if limit == q.NativeOperations() {
				if e != nil || out == nil {
					t.Fatal(e)
				}
			} else if e == nil || out != nil {
				t.Fatal("partial native Fuse")
			}
		}
		if out, e := proof.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{MaxVertices: 6}); e == nil || out != nil {
			t.Fatal("vertex limit")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if out, e := proof.BuildNative(ctx, trimmed.AffineArrangementPrismOptions{}); e != context.Canceled || out != nil {
			t.Fatal(out, e)
		}
	}
}
func TestNativeCrossingPrismFuseOriginalToolHole(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 3, Z: 3})
	b, e := brep.PrismWithHoles([]geom.Vec3{{X: 2, Y: .5, Z: 1}, {X: 5, Y: .5, Z: 1}, {X: 5, Y: 2.5, Z: 1}, {X: 2, Y: 2.5, Z: 1}}, [][]geom.Vec3{{{X: 2.5, Y: 1, Z: 1}, {X: 3, Y: 1, Z: 1}, {X: 3, Y: 2, Z: 1}, {X: 2.5, Y: 2, Z: 1}}}, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := trimmed.ImprintCrossingNativePrismFuse(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := proof.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s := q.Solid()
	m, e := s.MassProperties()
	if e != nil || math.Abs(m.Volume-45.5) > 1e-11 || math.Abs(m.Centroid.X-2027./728) > 1e-11 || math.Abs(m.Centroid.Y-1.5) > 1e-11 || math.Abs(m.Centroid.Z-325./182) > 1e-11 {
		t.Fatal(m, e)
	}
	if len(s.Vertices())-len(s.Edges())+len(s.Faces()) != 2 {
		t.Fatal("original hole should end at stock shoulder")
	}
	holeEdges := map[int]bool{}
	for _, u := range b.Faces()[0].Holes[0] {
		holeEdges[u.Edge] = true
	}
	n := 0
	for _, u := range q.WallUses() {
		if u.Use.Operand == 1 && holeEdges[u.Use.LowerEdge] {
			n++
			if q.Faces()[u.Use.Face].Region != 1 {
				t.Fatal("tool hole exposed in filled overlap")
			}
		}
	}
	if n != 4 {
		t.Fatal("original tool hole ancestry", n)
	}
}

func TestNativeCrossingPrismFuseNonbinaryRejected(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	b, e := brep.Prism([]geom.Vec3{{X: 1, Y: .5, Z: 1}, {X: 4, Y: 1, Z: 1}, {X: 1, Y: 1.5, Z: 1}}, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := trimmed.ImprintCrossingNativePrismFuse(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	nonbinary := false
	for _, v := range proof.UnionProfile().Vertices() {
		for _, p := range v.Point {
			_, exact := p.Float64()
			nonbinary = nonbinary || !exact
		}
	}
	if !nonbinary {
		t.Fatal("fixture lost exact nonbinary event")
	}
	if out, e := proof.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{}); e == nil || out != nil {
		t.Fatal("rounded native Fuse")
	}
}
