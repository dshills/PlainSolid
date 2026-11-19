//go:build research

package trimmed_test

import (
	"context"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestNativeSideOpeningPrismCut(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 3})
	for _, z := range [][2]float64{{-1, 4}, {0, 3}, {0, 4}, {-1, 3}} {
		b := transitionPrism(t, geom.Vec3{X: 1, Y: 0, Z: z[0]}, geom.Vec3{X: 3, Y: 2, Z: z[1]})
		q, e := trimmed.CutSideOpeningNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(z, e)
		}
		if len(q.Components()) != 1 {
			t.Fatal("components")
		}
		s := q.Components()[0]
		m, e := s.MassProperties()
		if e != nil || math.Abs(m.Volume-36) > 1e-10 || math.Abs(m.Centroid.X-31./12) > 1e-10 || math.Abs(m.Centroid.Y-7./3) > 1e-10 || math.Abs(m.Centroid.Z-1.5) > 1e-10 {
			t.Fatal(m, e)
		}
		if len(s.Vertices())-len(s.Edges())+len(s.Faces()) != 2 {
			t.Fatal("Euler")
		}
		toolWalls := map[int]bool{}
		for _, u := range q.Faces() {
			if u.Operand == 1 {
				if !u.Reversed {
					t.Fatal("cutter normal")
				}
				toolWalls[u.OriginalFace] = true
			}
		}
		if len(toolWalls) != 3 {
			t.Fatal("shared side survived", toolWalls)
		}
		r, e := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, trimmed.NativePrismDispatchOptions{})
		if e != nil {
			t.Fatal("dispatch", e)
		}
		if len(r.Components()) != 1 {
			t.Fatal("dispatch components")
		}
	}
}

func TestNativeSideOpeningPrismCutRejectAndLimits(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 3})
	for _, ends := range [][2]geom.Vec3{
		{{X: 1, Y: 1, Z: -1}, {X: 3, Y: 2, Z: 4}},
		{{X: 1, Y: -1, Z: -1}, {X: 3, Y: 2, Z: 4}},
		{{X: 0, Y: 0, Z: -1}, {X: 2, Y: 2, Z: 4}},
		{{X: 1, Y: 0, Z: 1}, {X: 3, Y: 2, Z: 4}},
	} {
		b := transitionPrism(t, ends[0], ends[1])
		if out, e := trimmed.CutSideOpeningNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
			t.Fatal("wrong domain", ends)
		}
	}
	tangent, e := brep.Prism([]geom.Vec3{{X: 2, Y: 0, Z: -1}, {X: 3, Y: 1, Z: -1}, {X: 2, Y: 2, Z: -1}, {X: 1, Y: 1, Z: -1}}, geom.Vec3{Z: 5})
	if e != nil {
		t.Fatal(e)
	}
	tangent, e = tangent.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	if out, e := trimmed.CutSideOpeningNativePrisms(context.Background(), a, tangent, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
		t.Fatal("isolated tangent promoted")
	}
	b := transitionPrism(t, geom.Vec3{X: 1, Y: 0, Z: -1}, geom.Vec3{X: 3, Y: 2, Z: 4})
	q, e := trimmed.CutSideOpeningNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, limit := range []int{q.NativeOperations(), q.NativeOperations() - 1} {
		o := trimmed.NativePrismBooleanOptions{}
		o.Native.MaxOperations = limit
		out, e := trimmed.CutSideOpeningNativePrisms(context.Background(), a, b, o)
		if limit == q.NativeOperations() {
			if e != nil {
				t.Fatal(e)
			}
		} else if e == nil || out != nil {
			t.Fatal("partial native")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := trimmed.CutSideOpeningNativePrisms(ctx, a, b, trimmed.NativePrismBooleanOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}
func TestNativeSideOpeningPrismCutUnsheared(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 3})
	b, _ := brep.Box(geom.Vec3{X: 1, Z: -1}, geom.Vec3{X: 3, Y: 2, Z: 4})
	q, e := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, trimmed.NativePrismDispatchOptions{})
	if e != nil || q.Family() != trimmed.NativePrismFamilySideOpeningCut {
		t.Fatal(q, e)
	}
}
