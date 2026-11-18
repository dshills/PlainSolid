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

func TestNativeSideOpeningPrismCutFloors(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 3})
	for _, span := range [][2]float64{{1, 4}, {-1, 2}, {1, 3}, {0, 2}} {
		b := transitionPrism(t, geom.Vec3{X: 1, Z: span[0]}, geom.Vec3{X: 3, Y: 2, Z: span[1]})
		p, e := trimmed.ImprintSideOpeningNativePrismCut(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(span, e)
		}
		q, e := p.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{})
		if e != nil {
			t.Fatal(span, e)
		}
		if len(q.Solid().Vertices())-len(q.Solid().Edges())+len(q.Solid().Faces()) != 2 {
			t.Fatal("Euler")
		}
		owners := [2]brep.Solid{a, b}
		floors := map[int]bool{}
		for _, f := range q.Faces() {
			verifySideOpeningFace(t, owners[f.Operand], q.Solid(), f.OriginalFace, f.Face, f.Reversed)
			if f.Region == 1 && f.Operand == 1 {
				floors[f.OriginalFace] = true
			}
		}
		if len(floors) != 1 {
			t.Fatal("original cutter floors", floors)
		}
		r, err := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, trimmed.NativePrismDispatchOptions{})
		if err != nil || len(r.Components()) != 1 {
			t.Fatal("dispatch", err)
		}
		m, e := q.Solid().MassProperties()
		wantZ := 1.4
		if span[0] <= 0 {
			wantZ = 1.6
		}
		if e != nil || math.Abs(m.Volume-40) > 1e-10 || math.Abs(m.Centroid.Z-wantZ) > 1e-10 {
			t.Fatal(m, e)
		}
	}
}
func TestNativeSideOpeningPrismCutInterior(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	b := transitionPrism(t, geom.Vec3{X: 1, Z: 1}, geom.Vec3{X: 3, Y: 2, Z: 3})
	p, e := trimmed.ImprintInteriorSideOpeningNativePrismCut(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := p.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Solid().Vertices())-len(q.Solid().Edges())+len(q.Solid().Faces()) != 2 {
		t.Fatal("Euler")
	}
	owners := [2]brep.Solid{a, b}
	floors := map[int]bool{}
	for _, f := range q.Faces() {
		verifySideOpeningFace(t, owners[f.Operand], q.Solid(), f.OriginalFace, f.Face, f.Reversed)
		if f.Region == 1 && f.Operand == 1 {
			floors[f.OriginalFace] = true
		}
	}
	if len(floors) != 2 {
		t.Fatal("original cutter floors", floors)
	}
	r, err := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, trimmed.NativePrismDispatchOptions{})
	if err != nil || len(r.Components()) != 1 {
		t.Fatal("dispatch", err)
	}
	m, e := q.Solid().MassProperties()
	if e != nil || math.Abs(m.Volume-56) > 1e-10 || math.Abs(m.Centroid.Z-2) > 1e-10 {
		t.Fatal(m, e)
	}
}

func verifySideOpeningFace(t *testing.T, src, dst brep.Solid, original, face int, reversed bool) {
	t.Helper()
	points := func(s brep.Solid, f brep.Face) []geom.Vec3 {
		out := []geom.Vec3{}
		v, e := s.Vertices(), s.Edges()
		for _, u := range f.Boundary {
			i := e[u.Edge].Start
			if u.Reversed {
				i = e[u.Edge].End
			}
			out = append(out, v[i])
		}
		return out
	}
	a, b := points(src, src.Faces()[original]), points(dst, dst.Faces()[face])
	n := a[1].Sub(a[0]).Cross(a[2].Sub(a[0]))
	m := b[1].Sub(b[0]).Cross(b[2].Sub(b[0]))
	sign := 1.
	if reversed {
		sign = -1
	}
	if sign*n.Dot(m) <= 0 {
		t.Fatal("source normal")
	}
	for _, v := range b {
		if n.Dot(v.Sub(a[0])) != 0 {
			t.Fatal("off original support")
		}
	}
}
func TestNativeSideOpeningPrismCutFloorLimits(t *testing.T) {
	ctx := context.Background()
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	b := transitionPrism(t, geom.Vec3{X: 1, Z: 1}, geom.Vec3{X: 3, Y: 2, Z: 3})
	p, e := trimmed.ImprintInteriorSideOpeningNativePrismCut(ctx, a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, aw := p.Operations()
	for _, limits := range [][2]int{{w, aw}, {w - 1, aw}, {w, aw - 1}} {
		o := trimmed.NativePrismBooleanOptions{}
		o.Native.MaxOperations = limits[0]
		o.Arrangement.MaxOperations = limits[1]
		out, e := trimmed.ImprintInteriorSideOpeningNativePrismCut(ctx, a, b, o)
		if limits[0] == w && limits[1] == aw {
			if e != nil {
				t.Fatal(e)
			}
		} else if e == nil || out != nil {
			t.Fatal("partial proof")
		}
	}
	q, e := p.BuildNative(ctx, trimmed.AffineArrangementPrismOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if out, e := p.BuildNative(ctx, trimmed.AffineArrangementPrismOptions{MaxOperations: q.NativeOperations() - 1}); e == nil || out != nil {
		t.Fatal("partial native")
	}
	for _, span := range [][2]float64{{1, 3}, {1, 5}} {
		bad := transitionPrism(t, geom.Vec3{Z: span[0]}, geom.Vec3{X: 2, Y: 2, Z: span[1]})
		if span[1] < 4 {
			if out, e := trimmed.ImprintInteriorSideOpeningNativePrismCut(ctx, a, bad, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
				t.Fatal("two-side interior")
			}
		} else {
			if out, e := trimmed.ImprintSideOpeningNativePrismCut(ctx, a, bad, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
				t.Fatal("two-side blind")
			}
		}
	}
}
func TestNativeSideOpeningPrismCutFloorTangency(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	for _, hi := range []float64{3, 5} {
		b, e := brep.Prism([]geom.Vec3{{X: 2, Z: 1}, {X: 3, Y: 1, Z: 1}, {X: 2, Y: 2, Z: 1}, {X: 1, Y: 1, Z: 1}}, geom.Vec3{Z: hi - 1})
		if e != nil {
			t.Fatal(e)
		}
		b, e = b.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
		if e != nil {
			t.Fatal(e)
		}
		if hi < 4 {
			if out, e := trimmed.ImprintInteriorSideOpeningNativePrismCut(context.Background(), a, b, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
				t.Fatal("interior isolated tangent")
			}
		} else {
			if out, e := trimmed.ImprintSideOpeningNativePrismCut(context.Background(), a, b, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
				t.Fatal("blind isolated tangent")
			}
		}
	}
}
