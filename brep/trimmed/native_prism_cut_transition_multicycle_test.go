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

func TestNativeCrossingPrismCutThreeRetainedTwoFloors(t *testing.T) {
	stock := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 2, Z: 3})
	for _, span := range [][2]float64{{1, 4}, {-1, 2}} {
		xy := [][2]float64{{.5, -2}, {3.5, -2}, {3.5, 3}, {2.5, 3}, {2.5, -1}, {1.5, -1}, {1.5, 3}, {.5, 3}}
		poles := []geom.Vec3{}
		for _, p := range xy {
			poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: span[0]})
		}
		tool, e := brep.Prism(poles, geom.Vec3{Z: span[1] - span[0]})
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
		if len(proof.Region(0).Cycles()) != 3 || len(proof.Region(1).Cycles()) != 2 {
			t.Fatal("missing upper components or floors")
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
		wantZ := 1.25
		if span[0] < 0 {
			wantZ = 1.75
		}
		if e != nil || math.Abs(m.Volume-16) > 1e-11 || math.Abs(m.Centroid.X-2.25) > 1e-12 || math.Abs(m.Centroid.Y-1) > 1e-12 || math.Abs(m.Centroid.Z-wantZ) > 1e-12 {
			t.Fatal(m, e)
		}
		// Independent graph connectedness and Euler characteristic distinguish one
		// solid bridge from accidentally disconnected retained upper slab solids.
		v, edges, faces := s.Vertices(), s.Edges(), s.Faces()
		if len(v)-len(edges)+len(faces) != 2 {
			t.Fatal("unexpected boundary topology")
		}
		seen := make([]bool, len(v))
		seen[0] = true
		queue := []int{0}
		for len(queue) > 0 {
			at := queue[0]
			queue = queue[1:]
			for _, e := range edges {
				next := -1
				if e.Start == at {
					next = e.End
				} else if e.End == at {
					next = e.Start
				}
				if next >= 0 && !seen[next] {
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}
		for _, yes := range seen {
			if !yes {
				t.Fatal("disconnected shell")
			}
		}
		capCount := [2]map[int]int{{}, {}}
		for _, u := range q.Faces() {
			if u.Edge == -1 && u.Region >= 0 {
				if u.Cycle < 0 || u.Cycle >= len(proof.Region(u.Region).Cycles()) {
					t.Fatal("lost cap cycle")
				}
				capCount[u.Region][u.Cycle]++
				owner := proof.CapOwners()[1]
				if u.Region == 1 {
					owner = proof.CapOwners()[0]
				}
				if u.Operand != owner.Operand || u.OriginalFace != owner.Face {
					t.Fatal("wrong original cap")
				}
			} else if u.Cycle != -1 {
				t.Fatal("wall has cap cycle")
			}
		}
		if len(capCount[0]) != 3 || len(capCount[1]) != 2 {
			t.Fatal(capCount)
		}
		for _, counts := range capCount {
			for _, n := range counts {
				if n != 2 {
					t.Fatal("rectangle cap triangulation", counts)
				}
			}
		}
		if _, e := brep.NewSolid(v, edges, faces[:len(faces)-1]); e == nil {
			t.Fatal("missing face accepted")
		}
		for _, limit := range []int{q.NativeOperations(), q.NativeOperations() - 1} {
			out, e := proof.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{MaxOperations: limit})
			if limit == q.NativeOperations() {
				if e != nil || out == nil {
					t.Fatal(e)
				}
			} else if e == nil || out != nil {
				t.Fatal("partial multi-cycle")
			}
		}
		changed := q.Faces()
		changed[0].Cycle = 999
		if q.Faces()[0].Cycle == 999 {
			t.Fatal("mutable cycle ancestry")
		}
	}
}
