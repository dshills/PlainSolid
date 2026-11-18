//go:build research

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

func TestNativeCoveringCrossingPrismCut(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	for _, span := range [][2]float64{{-1, 4}, {0, 4}, {-1, 3}, {0, 3}} {
		b := transitionPrism(t, geom.Vec3{X: 1, Y: .5, Z: span[0]}, geom.Vec3{X: 3, Y: 1.5, Z: span[1]})
		q, e := trimmed.CutCoveringCrossingNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(q.Components()) != 1 || q.Material().ExactVolume().Cmp(big.NewRat(9, 1)) != 0 {
			t.Fatal("wrong material")
		}
		s := q.Components()[0]
		m, e := s.MassProperties()
		if e != nil || math.Abs(m.Volume-9) > 1e-11 || math.Abs(m.Centroid.X-13./12) > 1e-12 || math.Abs(m.Centroid.Y-1) > 1e-12 || math.Abs(m.Centroid.Z-1.5) > 1e-12 {
			t.Fatal(m, e)
		}
		owners := [2]brep.Solid{a, b}
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
				t.Fatal("source reversal")
			}
			p, n := normal(owners[u.Operand], owners[u.Operand].Faces()[u.OriginalFace])
			_, got := normal(s, s.Faces()[u.Face])
			sign := 1.
			if u.Reversed {
				sign = -1
			}
			if n.Dot(got)*sign <= 0 {
				t.Fatal("normal")
			}
			for _, edge := range s.Faces()[u.Face].Boundary {
				v := s.Vertices()[s.Edges()[edge.Edge].Start]
				if n.Dot(v.Sub(p)) != 0 {
					t.Fatal("off source support")
				}
			}
		}
		for _, u := range q.WallUses() {
			if u.Use.Operand != 1 {
				continue
			}
			for i, z := range []float64{0, 3} {
				want := new(big.Rat).Quo(new(big.Rat).SetFloat64(z-span[0]), new(big.Rat).SetFloat64(span[1]-span[0]))
				if u.HeightParameter[i].Cmp(want) != 0 {
					t.Fatal("clipped original height", u)
				}
			}
		}
		if q.OriginalCaps()[0] != [2]int{0, 1} {
			t.Fatal(q.OriginalCaps())
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
			out, e := trimmed.CutCoveringCrossingNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxOperations: limit}})
			if limit == q.NativeOperations() {
				if e != nil || out == nil {
					t.Fatal(e)
				}
			} else if e == nil || out != nil {
				t.Fatal("partial covering Cut")
			}
		}
		if span != [2]float64{0, 3} {
			out, e := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, trimmed.NativePrismDispatchOptions{})
			if e != nil || out.Family() != trimmed.NativePrismFamilyCoveringCut || out.CoveringCutProof() == nil {
				t.Fatal("dispatcher", out, e)
			}
		}
	}
}
func TestNativeCoveringCrossingPrismCutComponentsAndPost(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	b := transitionPrism(t, geom.Vec3{X: -1, Y: .5, Z: -1}, geom.Vec3{X: 3, Y: 1.5, Z: 4})
	q, e := trimmed.CutCoveringCrossingNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Components()) != 2 || q.Material().ExactVolume().Cmp(big.NewRat(6, 1)) != 0 {
		t.Fatal("missing components")
	}
	sum := 0.
	for _, s := range q.Components() {
		m, e := s.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		sum += m.Volume
	}
	if math.Abs(sum-6) > 1e-11 {
		t.Fatal(sum)
	}
	if out, e := trimmed.CutCoveringCrossingNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxComponents: 1}}); e == nil || out != nil {
		t.Fatal("partial components")
	}
	a = transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 3, Z: 3})
	b, e = brep.PrismWithHoles([]geom.Vec3{{X: 2, Y: .5, Z: -1}, {X: 5, Y: .5, Z: -1}, {X: 5, Y: 2.5, Z: -1}, {X: 2, Y: 2.5, Z: -1}}, [][]geom.Vec3{{{X: 2.5, Y: 1, Z: -1}, {X: 3, Y: 1, Z: -1}, {X: 3, Y: 2, Z: -1}, {X: 2.5, Y: 2, Z: -1}}}, geom.Vec3{Z: 5})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	q, e = trimmed.CutCoveringCrossingNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Components()) != 2 || q.Material().ExactVolume().Cmp(big.NewRat(51, 2)) != 0 {
		t.Fatal("missing retained post")
	}
	sum = 0
	for _, s := range q.Components() {
		m, e := s.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		sum += m.Volume
	}
	if math.Abs(sum-25.5) > 1e-11 {
		t.Fatal(sum)
	}
	holeEdges := map[int]bool{}
	for _, u := range b.Faces()[0].Holes[0] {
		holeEdges[u.Edge] = true
	}
	n := 0
	for _, u := range q.WallUses() {
		if u.Use.Operand == 1 && holeEdges[u.Use.LowerEdge] {
			n++
		}
	}
	if n != 4 {
		t.Fatal("lost post ancestry", n)
	}
}
func TestNativeCoveringCrossingPrismCutReject(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	for _, ends := range [][2]geom.Vec3{{{X: 1, Y: .5, Z: 1}, {X: 3, Y: 1.5, Z: 4}}, {{X: 2, Y: .5, Z: -1}, {X: 3, Y: 1.5, Z: 4}}, {{X: .5, Y: .5, Z: -1}, {X: 1.5, Y: 1.5, Z: 4}}} {
		b := transitionPrism(t, ends[0], ends[1])
		if out, e := trimmed.CutCoveringCrossingNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
			t.Fatal("unsupported crossing", ends)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := trimmed.CutCoveringCrossingNativePrisms(ctx, a, a, trimmed.NativePrismBooleanOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}

func TestNativeCoveringCrossingPrismCutNonbinary(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	b, e := brep.Prism([]geom.Vec3{{X: 1, Y: .5, Z: -1}, {X: 4, Y: 1, Z: -1}, {X: 1, Y: 1.5, Z: -1}}, geom.Vec3{Z: 5})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	if out, e := trimmed.CutCoveringCrossingNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
		t.Fatal("rounded original crossing")
	}
}
