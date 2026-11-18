//go:build research

package trimmed_test

import (
	"context"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestNativeCrossingPrismCutClosed(t *testing.T) {
	stock := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	for _, span := range [][2]float64{{1, 4}, {-1, 2}} {
		tool := transitionPrism(t, geom.Vec3{X: 1, Y: .5, Z: span[0]}, geom.Vec3{X: 3, Y: 1.5, Z: span[1]})
		proof, e := trimmed.ImprintCrossingNativePrismCut(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{})
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
		if q.Transition() != proof {
			t.Fatal("unbound native result")
		}
		m, e := s.MassProperties()
		wantZ := 1.4
		if span[0] < 0 {
			wantZ = 1.6
		}
		if e != nil || math.Abs(m.Volume-10) > 1e-12 || math.Abs(m.Centroid.X-1.15) > 1e-12 || math.Abs(m.Centroid.Y-1) > 1e-12 || math.Abs(m.Centroid.Z-wantZ) > 1e-12 {
			t.Fatal(m, e)
		}
		if len(q.Faces()) != len(s.Faces()) || len(q.WallUses()) != 14 {
			t.Fatal("missing provenance", len(s.Faces()), len(q.WallUses()))
		}
		// Every emitted face lies on its original support plane, with orientation
		// reversed exactly for the tool. No triangle loses the original cap ancestry.
		owners := [2]brep.Solid{stock, tool}
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
		for _, u := range q.Faces() {
			p, n := normal(owners[u.Operand], owners[u.Operand].Faces()[u.OriginalFace])
			_, got := normal(s, s.Faces()[u.Face])
			sign := 1.
			if u.Reversed {
				sign = -1
			}
			if n.Dot(got)*sign <= 0 {
				t.Fatal("wrong source orientation", u)
			}
			for _, edge := range s.Faces()[u.Face].Boundary {
				p0 := s.Vertices()[s.Edges()[edge.Edge].Start]
				if n.Dot(p0.Sub(p)) != 0 {
					t.Fatal("off source support", u)
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
				t.Fatal("partial native")
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

func TestNativeCrossingPrismCutBuildNonbinaryAndMultipleCycles(t *testing.T) {
	stock := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	tool, e := brep.Prism([]geom.Vec3{{X: 1, Y: .5, Z: 1}, {X: 4, Y: 1, Z: 1}, {X: 1, Y: 1.5, Z: 1}}, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	tool, e = tool.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := trimmed.ImprintCrossingNativePrismCut(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	nonbinary := false
	for _, v := range proof.Region(1).Vertices() {
		for _, p := range v.Point {
			_, exact := p.Float64()
			nonbinary = nonbinary || !exact
		}
	}
	if !nonbinary {
		t.Fatal("fixture lost exact nonbinary crossing")
	}
	if out, e := proof.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{}); e == nil || out != nil {
		t.Fatal("rounded native crossing")
	}
	bar := transitionPrism(t, geom.Vec3{X: -1, Y: .5, Z: 1}, geom.Vec3{X: 3, Y: 1.5, Z: 4})
	proof, e = trimmed.ImprintCrossingNativePrismCut(context.Background(), stock, bar, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(proof.Region(0).Cycles()) != 2 {
		t.Fatal("missing separate retained components")
	}
	out, e := proof.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := out.Solid().MassProperties()
	if e != nil || math.Abs(m.Volume-8) > 1e-12 {
		t.Fatal(m, e)
	}
}
