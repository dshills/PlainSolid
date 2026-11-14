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

func TestNativeNestedCrossingPrismFuse(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
	b := transitionPrism(t, geom.Vec3{X: 1, Y: .5, Z: 1}, geom.Vec3{X: 3, Y: 1.5, Z: 3})
	for _, swap := range []bool{false, true} {
		owners := [2]brep.Solid{a, b}
		outer := 0
		if swap {
			owners = [2]brep.Solid{b, a}
			outer = 1
		}
		proof, e := trimmed.ImprintNestedCrossingNativePrismFuse(context.Background(), owners[0], owners[1], trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if proof.OuterOwner() != outer || proof.Axis() != 2 || proof.ExactMaterialVolume().Cmp(big.NewRat(18, 1)) != 0 {
			t.Fatal(proof.OuterOwner(), proof.ExactMaterialVolume())
		}
		for i, z := range []int64{0, 1, 3, 4} {
			if proof.Levels()[i].Cmp(big.NewRat(z, 1)) != 0 {
				t.Fatal(proof.Levels())
			}
		}
		shoulder := new(big.Rat)
		for _, c := range proof.ShoulderProfile().Cycles() {
			shoulder.Add(shoulder, c.SignedAreaA)
		}
		if shoulder.Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal(shoulder)
		}
		q, e := proof.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{})
		if e != nil {
			t.Fatal(e)
		}
		s := q.Solid()
		m, e := s.MassProperties()
		if e != nil || math.Abs(m.Volume-18) > 1e-11 || math.Abs(m.Centroid.X-17./12) > 1e-12 || math.Abs(m.Centroid.Y-1) > 1e-12 || math.Abs(m.Centroid.Z-2) > 1e-12 {
			t.Fatal(m, e)
		}
		if len(s.Vertices())-len(s.Edges())+len(s.Faces()) != 2 {
			t.Fatal("wrong nested shell topology")
		}
		shoulders := [2]bool{}
		for _, u := range q.Faces() {
			if u.Reversed {
				t.Fatal("reversed original support")
			}
			if u.Region == 1-outer && u.Edge == -1 && u.Cycle >= 0 {
				if u.OriginalFace == 0 {
					shoulders[0] = true
				} else if u.OriginalFace == 1 {
					shoulders[1] = true
				}
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
				t.Fatal("wrong original normal")
			}
			for _, edge := range s.Faces()[u.Face].Boundary {
				v := s.Vertices()[s.Edges()[edge.Edge].Start]
				if n.Dot(v.Sub(p)) != 0 {
					t.Fatal("off source support")
				}
			}
		}
		if !shoulders[0] || !shoulders[1] {
			t.Fatal("missing original inner caps")
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
		routed, e := trimmed.CombineNativePrisms(context.Background(), owners[0], owners[1], intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{})
		if e != nil || routed.Family() != trimmed.NativePrismFamilyNestedFuse || routed.NestedFuseProof() == nil {
			t.Fatal("previously unsupported route", routed, e)
		}
		nw, aw := proof.Operations()
		for _, mode := range []int{0, 1, 2} {
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
				p, e := trimmed.ImprintNestedCrossingNativePrismFuse(context.Background(), owners[0], owners[1], o)
				var out *trimmed.NativeNestedPrismFuse
				if e == nil {
					out, e = p.BuildNative(context.Background(), build)
				}
				if delta == 0 {
					if e != nil || out == nil {
						t.Fatal("exact quota", e)
					}
				} else if e == nil || out != nil {
					t.Fatal("partial nested result")
				}
			}
		}
	}
}
func TestNativeNestedCrossingPrismFuseReject(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
	for _, ends := range [][2]geom.Vec3{{{X: 1, Y: .5, Z: 0}, {X: 3, Y: 1.5, Z: 3}}, {{X: 1, Y: .5, Z: 1}, {X: 3, Y: 1.5, Z: 4}}, {{X: 1, Y: .5, Z: 4}, {X: 3, Y: 1.5, Z: 5}}, {{X: 2, Y: .5, Z: 1}, {X: 3, Y: 1.5, Z: 3}}, {{X: .5, Y: .5, Z: 1}, {X: 1.5, Y: 1.5, Z: 3}}} {
		b := transitionPrism(t, ends[0], ends[1])
		if out, e := trimmed.ImprintNestedCrossingNativePrismFuse(context.Background(), a, b, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
			t.Fatal("unsupported nested case", ends)
		}
	}
	b := transitionPrism(t, geom.Vec3{X: 1, Y: .5, Z: 1}, geom.Vec3{X: 3, Y: 1.5, Z: 3})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := trimmed.ImprintNestedCrossingNativePrismFuse(ctx, a, b, trimmed.NativePrismBooleanOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}

func TestNativeNestedCrossingPrismFuseHoleAndNonbinary(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 3, Z: 4})
	b, e := brep.PrismWithHoles([]geom.Vec3{{X: 2, Y: .5, Z: 1}, {X: 5, Y: .5, Z: 1}, {X: 5, Y: 2.5, Z: 1}, {X: 2, Y: 2.5, Z: 1}}, [][]geom.Vec3{{{X: 2.5, Y: 1, Z: 1}, {X: 3, Y: 1, Z: 1}, {X: 3, Y: 2, Z: 1}, {X: 2.5, Y: 2, Z: 1}}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	p, e := trimmed.ImprintNestedCrossingNativePrismFuse(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := p.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := q.Solid().MassProperties()
	if e != nil || math.Abs(m.Volume-52) > 1e-11 || math.Abs(m.Centroid.X-267./104) > 1e-11 || math.Abs(m.Centroid.Y-1.5) > 1e-11 || math.Abs(m.Centroid.Z-2) > 1e-11 {
		t.Fatal(m, e)
	}
	holeEdges := map[int]bool{}
	for _, u := range b.Faces()[0].Holes[0] {
		holeEdges[u.Edge] = true
	}
	for _, u := range q.WallUses() {
		if u.Use.Operand == 1 && holeEdges[u.Use.LowerEdge] {
			t.Fatal("filled inner hole exposed")
		}
	}
	a = transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
	b, e = brep.Prism([]geom.Vec3{{X: 1, Y: .5, Z: 1}, {X: 4, Y: 1, Z: 1}, {X: 1, Y: 1.5, Z: 1}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	p, e = trimmed.ImprintNestedCrossingNativePrismFuse(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	nonbinary := false
	for _, v := range p.UnionProfile().Vertices() {
		for _, c := range v.Point {
			_, exact := c.Float64()
			nonbinary = nonbinary || !exact
		}
	}
	if !nonbinary {
		t.Fatal("fixture lost nonbinary vertex")
	}
	if out, e := p.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{}); e == nil || out != nil {
		t.Fatal("rounded nested Fuse")
	}
}
