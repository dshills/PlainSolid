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

func TestNativeCrossingPrismCutRetainedAndFloorHoles(t *testing.T) {
	rect := func(x0, y0, x1, y1 float64) []geom.Vec3 {
		return []geom.Vec3{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
	}
	for _, tc := range []struct {
		x              float64
		region         int
		volume, cx, cz float64
	}{{.5, 0, 26.5, 909. / 424, 143. / 106}, {2.5, 1, 27.5, 911. / 440, 151. / 110}} {
		for _, bottom := range []bool{false, true} {
			stock, e := brep.PrismWithHoles(rect(0, 0, 4, 3), [][]geom.Vec3{rect(tc.x, 1, tc.x+.5, 2)}, geom.Vec3{Z: 3})
			if e != nil {
				t.Fatal(e)
			}
			stock, e = stock.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
			if e != nil {
				t.Fatal(e)
			}
			lo, hi := 1., 4.
			if bottom {
				lo, hi = -1, 2
			}
			tool := transitionPrism(t, geom.Vec3{X: 2, Y: .5, Z: lo}, geom.Vec3{X: 5, Y: 2.5, Z: hi})
			proof, e := trimmed.ImprintCrossingNativePrismCut(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{})
			if e != nil {
				t.Fatal(e)
			}
			holes := 0
			for _, c := range proof.Region(tc.region).Cycles() {
				if c.SignedAreaA.Sign() < 0 {
					holes++
				}
			}
			if holes != 1 {
				t.Fatal("fixture did not preserve hole", tc.region, holes)
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
			cz := tc.cz
			if bottom {
				cz = 3 - cz
			}
			if e != nil || math.Abs(m.Volume-tc.volume) > 1e-11 || math.Abs(m.Centroid.X-tc.cx) > 1e-11 || math.Abs(m.Centroid.Y-1.5) > 1e-11 || math.Abs(m.Centroid.Z-cz) > 1e-11 {
				t.Fatal(tc, bottom, m, e)
			}
			if len(s.Vertices())-len(s.Edges())+len(s.Faces()) != 0 {
				t.Fatal("lost original through-hole topology")
			}
			// Original hole wall IDs must survive wherever the cut does not remove them.
			holeEdges := map[int]bool{}
			for _, u := range stock.Faces()[0].Holes[0] {
				holeEdges[u.Edge] = true
			}
			holeUses := 0
			for _, u := range q.WallUses() {
				if u.Use.Operand == 0 && holeEdges[u.Use.LowerEdge] {
					holeUses++
				}
			}
			want := 8
			if tc.region == 1 {
				want = 4
			}
			if holeUses != want {
				t.Fatal("original hole incidences", holeUses, want)
			}
			for _, u := range q.Faces() {
				if u.Region >= 0 && u.Edge == -1 {
					if proof.Region(u.Region).Cycles()[u.Cycle].SignedAreaA.Sign() <= 0 {
						t.Fatal("cap triangle assigned to excluded hole")
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
			if !bottom {
				for _, limit := range []int{q.NativeOperations(), q.NativeOperations() - 1} {
					out, e := proof.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{MaxOperations: limit})
					if limit == q.NativeOperations() {
						if e != nil || out == nil {
							t.Fatal(e)
						}
					} else if e == nil || out != nil {
						t.Fatal("partial holed cap")
					}
				}
			}
		}
	}
}

func TestNativeCrossingPrismCutToolHoleRetainsIsland(t *testing.T) {
	stock := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 3, Z: 3})
	tool, e := brep.PrismWithHoles([]geom.Vec3{{X: 2, Y: .5, Z: 1}, {X: 5, Y: .5, Z: 1}, {X: 5, Y: 2.5, Z: 1}, {X: 2, Y: 2.5, Z: 1}}, [][]geom.Vec3{{{X: 2.5, Y: 1, Z: 1}, {X: 3, Y: 1, Z: 1}, {X: 3, Y: 2, Z: 1}, {X: 2.5, Y: 2, Z: 1}}}, geom.Vec3{Z: 3})
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
	if len(proof.Region(0).Cycles()) != 2 || len(proof.Region(1).Cycles()) != 2 {
		t.Fatal("missing retained island/floor hole")
	}
	q, e := proof.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s := q.Solid()
	m, e := s.MassProperties()
	if e != nil || math.Abs(m.Volume-29) > 1e-11 || math.Abs(m.Centroid.X-493./232) > 1e-11 || math.Abs(m.Centroid.Y-1.5) > 1e-11 || math.Abs(m.Centroid.Z-40./29) > 1e-11 {
		t.Fatal(m, e)
	}
	if len(s.Vertices())-len(s.Edges())+len(s.Faces()) != 2 {
		t.Fatal("island not connected through stock")
	}
	holeEdges := map[int]bool{}
	for _, u := range tool.Faces()[0].Holes[0] {
		holeEdges[u.Edge] = true
	}
	n := 0
	for _, u := range q.WallUses() {
		if u.Use.Operand == 1 && holeEdges[u.Use.LowerEdge] {
			n++
			if !q.Faces()[u.Use.Face].Reversed {
				t.Fatal("tool hole orientation")
			}
		}
	}
	if n != 4 {
		t.Fatal("missing original tool hole walls", n)
	}
}
