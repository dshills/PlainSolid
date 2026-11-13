package faceclip

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func holed(t *testing.T) brep.Solid {
	t.Helper()
	s, e := brep.PrismWithHoles([]geom.Vec3{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}, [][]geom.Vec3{{{X: 1, Y: 1}, {X: 1, Y: 3}, {X: 3, Y: 3}, {X: 3, Y: 1}}}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestClipHoleAreaCentroid(t *testing.T) {
	s := holed(t)
	r, e := Clip(context.Background(), s, 1, Halfspace{geom.Vec3{X: 1}, 2}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.ProjectedArea.Cmp(big.NewRat(6, 1)) != 0 || r.Centroid[0].Cmp(big.NewRat(5, 6)) != 0 || r.Centroid[1].Cmp(big.NewRat(2, 1)) != 0 || r.Centroid[2].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(r.ProjectedArea, r.Centroid)
	}
	for _, f := range r.Fragments {
		if f.Face != 1 || s.TriangleFaces()[f.Triangle] != 1 {
			t.Fatal("ancestry")
		}
		for _, p := range f.Vertices {
			if p[0].Cmp(big.NewRat(2, 1)) > 0 {
				t.Fatal("outside")
			}
		}
	}
	other, e := Clip(context.Background(), s, 1, Halfspace{geom.Vec3{X: -1}, -2}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if add(r.ProjectedArea, other.ProjectedArea).Cmp(big.NewRat(12, 1)) != 0 {
		t.Fatal("area conservation")
	}
}
func TestRationalTriangleAndEmpty(t *testing.T) {
	s := holed(t)
	r, e := Clip(context.Background(), s, 1, Halfspace{geom.Vec3{X: 1, Y: 1}, 1}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.ProjectedArea.Cmp(big.NewRat(1, 2)) != 0 || r.Centroid[0].Cmp(big.NewRat(1, 3)) != 0 || r.Centroid[1].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(r)
	}
	for _, d := range []float64{0, -1} {
		r, e = Clip(context.Background(), s, 1, Halfspace{geom.Vec3{X: 1}, d}, Options{})
		if e != nil || len(r.Fragments) != 0 || r.ProjectedArea.Sign() != 0 || r.Centroid[0] != nil {
			t.Fatal(e, r)
		}
	}
}
func TestObliqueSurfaceArea(t *testing.T) {
	s := holed(t)
	a := geom.Identity()
	a[2][0] = .5
	s, e := s.Transformed(a)
	if e != nil {
		t.Fatal(e)
	}
	r, e := Clip(context.Background(), s, 1, Halfspace{geom.Vec3{X: 1}, 2}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.ProjectedArea.Cmp(big.NewRat(6, 1)) != 0 || math.Abs(r.Area-3*math.Sqrt(5)) > 1e-12 || r.Centroid[2].Cmp(big.NewRat(17, 12)) != 0 {
		t.Fatal(r.Area, r.Centroid)
	}
}
func TestClipBudgets(t *testing.T) {
	s := holed(t)
	for _, o := range []Options{{MaxWork: 1}, {MaxTriangles: 1}, {MaxBits: 1}} {
		if _, e := Clip(context.Background(), s, 1, Halfspace{geom.Vec3{X: 1}, 2}, o); !errors.Is(e, ErrLimit) {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Clip(ctx, s, 1, Halfspace{geom.Vec3{X: 1}, 2}, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func FuzzHoleAreaConservation(f *testing.F) {
	f.Add(int16(1), int16(2), int16(3))
	f.Fuzz(func(t *testing.T, a, b, d int16) {
		if a == 0 && b == 0 {
			return
		}
		s := holed(t)
		h := Halfspace{geom.Vec3{X: float64(a % 16), Y: float64(b % 16)}, float64(d%32) / 8}
		if h.Normal == (geom.Vec3{}) {
			return
		}
		r, e := Clip(context.Background(), s, 1, h, Options{})
		if e != nil {
			t.Fatal(e)
		}
		h.Normal = h.Normal.Scale(-1)
		h.Offset = -h.Offset
		q, e := Clip(context.Background(), s, 1, h, Options{})
		if e != nil {
			t.Fatal(e)
		}
		if add(r.ProjectedArea, q.ProjectedArea).Cmp(big.NewRat(12, 1)) != 0 {
			t.Fatal("area not conserved")
		}
	})
}
