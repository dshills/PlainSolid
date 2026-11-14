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

func TestNativeProfileCoveredPrismCut(t *testing.T) {
	for _, ends := range [][2]float64{{1, 3}, {-1, 2}, {2, 5}, {0, 2}, {2, 4}} {
		a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
		b := transitionPrism(t, geom.Vec3{X: -1, Y: -1, Z: ends[0]}, geom.Vec3{X: 3, Y: 3, Z: ends[1]})
		q, e := trimmed.CutProfileCoveredNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(ends, e)
		}
		n := 1
		if ends[0] > 0 && ends[1] < 4 {
			n = 2
		}
		if len(q.Components()) != n {
			t.Fatal("slabs")
		}
		vol := 0.
		for _, s := range q.Components() {
			m, e := s.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			vol += m.Volume
			if len(s.Vertices())-len(s.Edges())+len(s.Faces()) != 2 {
				t.Fatal("Euler")
			}
		}
		want := 4 * (4 - math.Min(4, ends[1]) + math.Max(0, ends[0]))
		if math.Abs(vol-want) > 1e-10 {
			t.Fatal(vol, want)
		}
		forming := 0
		for _, f := range q.Faces() {
			owners := [2]brep.Solid{a, b}
			src := owners[f.Operand]
			dst := q.Components()[f.Component]
			points := func(s brep.Solid, face brep.Face) []geom.Vec3 {
				out := []geom.Vec3{}
				vs, es := s.Vertices(), s.Edges()
				for _, u := range face.Boundary {
					i := es[u.Edge].Start
					if u.Reversed {
						i = es[u.Edge].End
					}
					out = append(out, vs[i])
				}
				return out
			}
			p, r := points(src, src.Faces()[f.OriginalFace]), points(dst, dst.Faces()[f.Face])
			n := p[1].Sub(p[0]).Cross(p[2].Sub(p[0]))
			rn := r[1].Sub(r[0]).Cross(r[2].Sub(r[0]))
			sign := 1.
			if f.Reversed {
				sign = -1
			}
			if sign*n.Dot(rn) <= 0 {
				t.Fatal("cap/wall orientation")
			}
			for _, v := range r {
				if n.Dot(v.Sub(p[0])) != 0 {
					t.Fatal("source support")
				}
			}
			if f.Operand == 1 {
				forming++
				if !f.Reversed {
					t.Fatal("tool cap not reversed")
				}
			}
		}
		if forming != n {
			t.Fatal("forming caps", forming, n)
		}
		for _, u := range q.WallUses() {
			if u.Use.Operand != 0 || u.HeightParameter[0].Cmp(u.HeightParameter[1]) >= 0 {
				t.Fatal("stock wall history")
			}
		}
		r, e := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, trimmed.NativePrismDispatchOptions{})
		if e != nil || r.Family() != trimmed.NativePrismFamilyProfileCoveredCut {
			t.Fatal("dispatch", e)
		}
	}
}
func TestNativeProfileCoveredPrismCutHole(t *testing.T) {
	a, e := brep.PrismWithHoles([]geom.Vec3{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}, [][]geom.Vec3{{{X: 1, Y: 1}, {X: 3, Y: 1}, {X: 3, Y: 3}, {X: 1, Y: 3}}}, geom.Vec3{Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	a, e = a.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	b := transitionPrism(t, geom.Vec3{X: -1, Y: -1, Z: 1}, geom.Vec3{X: 5, Y: 5, Z: 3})
	q, e := trimmed.CutProfileCoveredNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Components()) != 2 {
		t.Fatal("disconnected slabs")
	}
	for _, s := range q.Components() {
		m, e := s.MassProperties()
		holes := 0
		for _, f := range s.Faces() {
			holes += len(f.Holes)
		}
		if e != nil || math.Abs(m.Volume-12) > 1e-10 || len(s.Vertices())-len(s.Edges())+len(s.Faces())-holes != 0 {
			t.Fatal(m, e)
		}
	}
	o := trimmed.NativePrismBooleanOptions{}
	o.Native.MaxComponents = 1
	if out, e := trimmed.CutProfileCoveredNativePrisms(context.Background(), a, b, o); e == nil || out != nil {
		t.Fatal("partial components")
	}
	o = trimmed.NativePrismBooleanOptions{}
	o.Native.MaxOperations = q.NativeOperations()
	if _, e := trimmed.CutProfileCoveredNativePrisms(context.Background(), a, b, o); e != nil {
		t.Fatal("exact cumulative budget", e)
	}
	o.Native.MaxOperations = q.NativeOperations() - 1
	if out, e := trimmed.CutProfileCoveredNativePrisms(context.Background(), a, b, o); e == nil || out != nil {
		t.Fatal("partial quota")
	}
}
func TestNativeProfileCoveredPrismCutUnshearedAndEqual(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
	for _, equal := range []bool{false, true} {
		lo, hi := -1., 3.
		if equal {
			lo, hi = 0, 2
		}
		b, _ := brep.Box(geom.Vec3{X: lo, Y: lo, Z: 1}, geom.Vec3{X: hi, Y: hi, Z: 3})
		q, e := trimmed.CutProfileCoveredNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
		if e != nil || len(q.Components()) != 2 {
			t.Fatal(q, e)
		}
		r, e := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, trimmed.NativePrismDispatchOptions{})
		if e != nil || len(r.Components()) != 2 {
			t.Fatal("dispatch", r, e)
		}
	}
}
func TestNativeProfileCoveredPrismCutReject(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 4})
	for _, ends := range [][2]geom.Vec3{
		{{X: 1, Y: -1, Z: 1}, {X: 3, Y: 3, Z: 3}},
		{{X: -1, Y: -1, Z: 4}, {X: 3, Y: 3, Z: 5}},
		{{X: -1, Y: -1, Z: -1}, {X: 3, Y: 3, Z: 5}},
	} {
		b := transitionPrism(t, ends[0], ends[1])
		if out, e := trimmed.CutProfileCoveredNativePrisms(context.Background(), a, b, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
			t.Fatal("wrong family accepted")
		}
	}
	b := transitionPrism(t, geom.Vec3{X: -1, Y: -1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 3})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := trimmed.CutProfileCoveredNativePrisms(ctx, a, b, trimmed.NativePrismBooleanOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}
