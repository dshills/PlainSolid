package trimmed_test

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestNativeNestedCrossingPrismCut(t *testing.T) {
	for _, mirror := range []bool{false, true} {
		a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
		b := transitionPrism(t, geom.Vec3{X: 1, Y: .5, Z: 1}, geom.Vec3{X: 3, Y: 1.5, Z: 3})
		wantZ := 2.
		if mirror {
			pose := geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, -1, 7}}
			var e error
			a, e = a.Transformed(pose)
			if e != nil {
				t.Fatal(e)
			}
			b, e = b.Transformed(pose)
			if e != nil {
				t.Fatal(e)
			}
			wantZ = 5
		}
		p, e := trimmed.ImprintNestedCrossingNativePrismCut(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if p.ExactMaterialVolume().Cmp(big.NewRat(14, 1)) != 0 {
			t.Fatal(p.ExactMaterialVolume())
		}
		q, e := p.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{})
		if e != nil {
			t.Fatal(e)
		}
		s := q.Solid()
		m, e := s.MassProperties()
		if e != nil || math.Abs(m.Volume-14) > 1e-11 || math.Abs(m.Centroid.X-33./28) > 1e-12 || math.Abs(m.Centroid.Y-1) > 1e-12 || math.Abs(m.Centroid.Z-wantZ) > 1e-12 {
			t.Fatal(m, e)
		}
		if len(s.Vertices())-len(s.Edges())+len(s.Faces()) != 2 {
			t.Fatal("unexpected native topology")
		}
		owners := [2]brep.Solid{a, b}
		floors := map[int]bool{}
		normal := func(s brep.Solid, f brep.Face) (geom.Vec3, geom.Vec3) {
			v, edges := s.Vertices(), s.Edges()
			pts := []geom.Vec3{}
			for _, u := range f.Boundary {
				edge := edges[u.Edge]
				id := edge.Start
				if u.Reversed {
					id = edge.End
				}
				pts = append(pts, v[id])
			}
			return pts[0], pts[1].Sub(pts[0]).Cross(pts[2].Sub(pts[0]))
		}
		for _, u := range q.Faces() {
			if u.Reversed != (u.Operand == 1) {
				t.Fatal("wrong source reversal")
			}
			if u.Region == 1 && u.Edge == -1 {
				floors[u.OriginalFace] = true
			}
			point, n := normal(owners[u.Operand], owners[u.Operand].Faces()[u.OriginalFace])
			_, got := normal(s, s.Faces()[u.Face])
			sign := 1.
			if u.Reversed {
				sign = -1
			}
			if n.Dot(got)*sign <= 0 {
				t.Fatal("wrong normal")
			}
			for _, edge := range s.Faces()[u.Face].Boundary {
				v := s.Vertices()[s.Edges()[edge.Edge].Start]
				if n.Dot(v.Sub(point)) != 0 {
					t.Fatal("off original support")
				}
			}
		}
		if len(floors) != 2 {
			t.Fatal("missing original cutter cap")
		}
		for _, u := range q.WallUses() {
			for _, h := range u.HeightParameter {
				if h.Sign() < 0 || h.Cmp(big.NewRat(1, 1)) > 0 {
					t.Fatal("outside original wall")
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
		out, e := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, trimmed.NativePrismDispatchOptions{})
		if e != nil || out.Family() != trimmed.NativePrismFamilyNestedCut || out.NestedCutProof() == nil {
			t.Fatal("normal routing", out, e)
		}
		nw, aw := p.Operations()
		for mode := 0; mode < 3; mode++ {
			for _, delta := range []int{0, -1} {
				o := trimmed.NativePrismBooleanOptions{}
				build := trimmed.AffineArrangementPrismOptions{}
				if mode == 0 {
					o.Native.MaxOperations = nw + delta
				} else if mode == 1 {
					o.Arrangement.MaxOperations = aw + delta
				} else {
					build.MaxOperations = q.NativeOperations() + delta
				}
				proof, e := trimmed.ImprintNestedCrossingNativePrismCut(context.Background(), a, b, o)
				var out *trimmed.NativeNestedPrismCut
				if e == nil {
					out, e = proof.BuildNative(context.Background(), build)
				}
				if delta == 0 {
					if e != nil || out == nil {
						t.Fatal("exact limit", e)
					}
				} else if e == nil || out != nil {
					t.Fatal("partial Cut")
				}
			}
		}
		if out, e := trimmed.ImprintNestedCrossingNativePrismCut(context.Background(), b, a, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
			t.Fatal("reversed containment")
		}
	}
}
func TestNativeNestedCrossingPrismCutToolHole(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 3, Z: 4})
	b, e := brep.PrismWithHoles([]geom.Vec3{{X: 2, Y: .5, Z: 1}, {X: 5, Y: .5, Z: 1}, {X: 5, Y: 2.5, Z: 1}, {X: 2, Y: 2.5, Z: 1}}, [][]geom.Vec3{{{X: 2.5, Y: 1, Z: 1}, {X: 3, Y: 1, Z: 1}, {X: 3, Y: 2, Z: 1}, {X: 2.5, Y: 2, Z: 1}}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	p, e := trimmed.ImprintNestedCrossingNativePrismCut(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := p.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s := q.Solid()
	m, e := s.MassProperties()
	if e != nil || math.Abs(m.Volume-41) > 1e-11 || math.Abs(m.Centroid.X-721./328) > 1e-11 || math.Abs(m.Centroid.Y-1.5) > 1e-11 || math.Abs(m.Centroid.Z-2) > 1e-11 {
		t.Fatal(m, e)
	}
	if len(s.Vertices())-len(s.Edges())+len(s.Faces()) != 0 {
		t.Fatal("retained connecting post topology")
	}
	holeEdges := map[int]bool{}
	for _, u := range b.Faces()[0].Holes[0] {
		holeEdges[u.Edge] = true
	}
	n := 0
	for _, u := range q.WallUses() {
		if u.Use.Operand == 1 && holeEdges[u.Use.LowerEdge] {
			n++
			if !q.Faces()[u.Use.Face].Reversed {
				t.Fatal("tool hole wall not reversed")
			}
		}
	}
	if n != 4 {
		t.Fatal("lost retained tool hole", n)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := p.BuildNative(ctx, trimmed.AffineArrangementPrismOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}

func TestNativeNestedCrossingPrismCutReject(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
	for _, ends := range [][2]geom.Vec3{{{X: 1, Y: .5, Z: 0}, {X: 3, Y: 1.5, Z: 3}}, {{X: 1, Y: .5, Z: 1}, {X: 3, Y: 1.5, Z: 4}}, {{X: 2, Y: .5, Z: 1}, {X: 3, Y: 1.5, Z: 3}}, {{X: .5, Y: .5, Z: 1}, {X: 1.5, Y: 1.5, Z: 3}}} {
		b := transitionPrism(t, ends[0], ends[1])
		if out, e := trimmed.ImprintNestedCrossingNativePrismCut(context.Background(), a, b, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
			t.Fatal("unsupported interior cut", ends)
		}
	}
	b, e := brep.Prism([]geom.Vec3{{X: 1, Y: .5, Z: 1}, {X: 4, Y: 1, Z: 1}, {X: 1, Y: 1.5, Z: 1}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	p, e := trimmed.ImprintNestedCrossingNativePrismCut(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	nonbinary := false
	for _, v := range p.FloorProfile().Vertices() {
		for _, q := range v.Point {
			_, exact := q.Float64()
			nonbinary = nonbinary || !exact
		}
	}
	if !nonbinary {
		t.Fatal("lost exact nonbinary fixture")
	}
	if out, e := p.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{}); e == nil || out != nil {
		t.Fatal("rounded native cut")
	}
}
