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

func TestNativeAbuttingCrossingPrismFuse(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	b := transitionPrism(t, geom.Vec3{X: 1, Y: .5, Z: 3}, geom.Vec3{X: 3, Y: 1.5, Z: 5})
	for _, swap := range []bool{false, true} {
		owners := [2]brep.Solid{a, b}
		lower := 0
		if swap {
			owners = [2]brep.Solid{b, a}
			lower = 1
		}
		p, e := trimmed.ImprintAbuttingCrossingNativePrismFuse(context.Background(), owners[0], owners[1], trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if p.LowerOwner() != lower || p.ExactMaterialVolume().Cmp(big.NewRat(16, 1)) != 0 {
			t.Fatal("wrong positive-area join")
		}
		for i, z := range []int64{0, 3, 5} {
			if p.Levels()[i].Cmp(big.NewRat(z, 1)) != 0 {
				t.Fatal(p.Levels())
			}
		}
		q, e := p.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{})
		if e != nil {
			t.Fatal(e)
		}
		s := q.Solid()
		m, e := s.MassProperties()
		if e != nil || math.Abs(m.Volume-16) > 1e-11 || math.Abs(m.Centroid.X-1.5) > 1e-12 || math.Abs(m.Centroid.Y-1) > 1e-12 || math.Abs(m.Centroid.Z-17./8) > 1e-12 {
			t.Fatal(m, e)
		}
		if len(s.Vertices())-len(s.Edges())+len(s.Faces()) != 2 {
			t.Fatal("wrong join topology")
		}
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
		joinArea := 0.
		capOwners := [2]bool{}
		for _, u := range q.Faces() {
			if u.Reversed {
				t.Fatal("unexpected original reversal")
			}
			point, n := normal(owners[u.Operand], owners[u.Operand].Faces()[u.OriginalFace])
			_, got := normal(s, s.Faces()[u.Face])
			if n.Dot(got) <= 0 {
				t.Fatal("wrong support normal")
			}
			for _, edge := range s.Faces()[u.Face].Boundary {
				v := s.Vertices()[s.Edges()[edge.Edge].Start]
				if n.Dot(v.Sub(point)) != 0 {
					t.Fatal("off source support")
				}
			}
			if u.Region >= 0 && u.Cycle >= 0 {
				capOwners[u.Operand] = true
				joinArea += math.Abs(got.Z) / 2
			}
		}
		if !capOwners[0] || !capOwners[1] || math.Abs(joinArea-4) > 1e-12 {
			t.Fatal("shared Common cap was not canceled", joinArea)
		}
		for _, u := range q.WallUses() {
			if u.HeightParameter[0].Sign() != 0 || u.HeightParameter[1].Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal("changed original wall band")
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
		out, e := trimmed.CombineNativePrisms(context.Background(), owners[0], owners[1], intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{})
		if e != nil || out.Family() != trimmed.NativePrismFamilyAbuttingFuse || out.AbuttingFuseProof() == nil {
			t.Fatal("routing", out, e)
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
				proof, e := trimmed.ImprintAbuttingCrossingNativePrismFuse(context.Background(), owners[0], owners[1], o)
				var out *trimmed.NativeAbuttingPrismFuse
				if e == nil {
					out, e = proof.BuildNative(context.Background(), build)
				}
				if delta == 0 {
					if e != nil || out == nil {
						t.Fatal("exact quota", e)
					}
				} else if e == nil || out != nil {
					t.Fatal("partial cap join")
				}
			}
		}
	}
}
func TestNativeAbuttingCrossingPrismFuseRejectAndUnsheared(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	b, _ := brep.Box(geom.Vec3{X: 1, Y: .5, Z: 3}, geom.Vec3{X: 3, Y: 1.5, Z: 5})
	q, e := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{})
	if e != nil || q.Family() != trimmed.NativePrismFamilyAbuttingFuse {
		t.Fatal("unsheared routing", q, e)
	}
	for _, ends := range [][2]geom.Vec3{{{X: 2, Y: .5, Z: 3}, {X: 3, Y: 1.5, Z: 5}}, {{X: 2, Y: 2, Z: 3}, {X: 3, Y: 3, Z: 5}}, {{X: 1, Y: .5, Z: math.Nextafter(3, 4)}, {X: 3, Y: 1.5, Z: 5}}, {{X: 1, Y: .5, Z: math.Nextafter(3, 2)}, {X: 3, Y: 1.5, Z: 5}}} {
		bad, _ := brep.Box(ends[0], ends[1])
		if out, e := trimmed.ImprintAbuttingCrossingNativePrismFuse(context.Background(), a, bad, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
			t.Fatal("unrelated/non-area contact", ends)
		}
	}
	for _, ends := range [][2]geom.Vec3{{{X: 2, Y: .5, Z: 3}, {X: 3, Y: 1.5, Z: 5}}, {{X: 2, Y: 2, Z: 3}, {X: 3, Y: 3, Z: 5}}} {
		bad, _ := brep.Box(ends[0], ends[1])
		if out, e := trimmed.CombineNativePrisms(context.Background(), a, bad, intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{}); e == nil || out != nil {
			t.Fatal("line/point contact promoted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := trimmed.ImprintAbuttingCrossingNativePrismFuse(ctx, a, b, trimmed.NativePrismBooleanOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}

func TestNativeAbuttingCrossingPrismFuseNonbinary(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	b, e := brep.Prism([]geom.Vec3{{X: 1, Y: .5, Z: 3}, {X: 4, Y: 1, Z: 3}, {X: 1, Y: 1.5, Z: 3}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	p, e := trimmed.ImprintAbuttingCrossingNativePrismFuse(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
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
		t.Fatal("rounded abutting Fuse")
	}
}
