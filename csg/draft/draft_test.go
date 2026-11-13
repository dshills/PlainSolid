package draft

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func cube(t *testing.T) brep.Solid {
	t.Helper()
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func neutral(t *testing.T) geom.Plane {
	t.Helper()
	p, e := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func checkVolume(t *testing.T, r Result, n, d int64) {
	t.Helper()
	want := big.NewRat(n, d)
	if r.ExactVolume.Cmp(want) != 0 {
		t.Fatalf("volume %v want %v", r.ExactVolume, want)
	}
	if len(r.Solid.Faces()) > 0 {
		m, e := r.Solid.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		v, _ := want.Float64()
		if math.Abs(m.Volume-v) > 1e-12 {
			t.Fatalf("mesh volume %g want %g", m.Volume, v)
		}
	}
}
func TestSingleFace(t *testing.T) {
	s := cube(t)
	r, e := Shear(context.Background(), s, neutral(t), []FaceShear{{3, geom.Vec3{X: .5}}}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	checkVolume(t, r, 5, 4)
	for i, p := range r.ExactVertices {
		for _, h := range r.FacePlanes {
			if dot(p, h.Normal).Cmp(h.Offset) > 0 {
				t.Fatalf("vertex %d outside", i)
			}
		}
	}
	if len(r.FaceSources) != 6 {
		t.Fatal(r.FaceSources)
	}
}
func TestFourFacesTopologyChange(t *testing.T) {
	for _, tc := range []struct {
		k     float64
		n, d  int64
		faces int
	}{{.25, 19, 12, 6}, {-.75, 2, 9, 5}} {
		r, e := Shear(context.Background(), cube(t), neutral(t), []FaceShear{{5, geom.Vec3{X: -tc.k}}, {3, geom.Vec3{X: tc.k}}, {2, geom.Vec3{Y: -tc.k}}, {4, geom.Vec3{Y: tc.k}}}, Options{})
		if e != nil {
			t.Fatal(e)
		}
		checkVolume(t, r, tc.n, tc.d)
		if len(r.Solid.Faces()) != tc.faces {
			t.Fatal(len(r.Solid.Faces()))
		}
		if tc.k < 0 && r.OldToNew[1] != -1 {
			t.Fatal("top should vanish")
		}
	}
}
func TestObliqueNeutral(t *testing.T) {
	p, e := geom.NewPlane(geom.Vec3{Z: .5}, geom.Vec3{X: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	r, e := Shear(context.Background(), cube(t), p, []FaceShear{{3, geom.Vec3{X: .25}}}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	want := add(big.NewRat(1, 1), mul(big.NewRat(1, 4), num(p.Normal().X)))
	if r.ExactVolume.Cmp(want) != 0 {
		t.Fatalf("%v want %v", r.ExactVolume, want)
	}
}
func TestAffineReflectionAndEmpty(t *testing.T) {
	s := cube(t)
	reflect := geom.Identity()
	reflect[0][0] = -1
	var all []FaceTransform
	for i := 0; i < 6; i++ {
		all = append(all, FaceTransform{i, reflect})
	}
	r, e := Apply(context.Background(), s, all, Options{})
	if e != nil {
		t.Fatal(e)
	}
	checkVolume(t, r, 1, 1)
	_, e = Apply(context.Background(), s, all[3:4], Options{})
	if !errors.Is(e, ErrUnbounded) {
		t.Fatal(e)
	}
	move := geom.Identity()
	move[0][3] = -2
	r, e = Apply(context.Background(), s, []FaceTransform{{3, move}}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	checkVolume(t, r, 0, 1)
	if len(r.Solid.Faces()) != 0 {
		t.Fatal("expected empty")
	}
}
func TestFailures(t *testing.T) {
	s := cube(t)
	for _, o := range []Options{{MaxWork: 1}, {MaxPlanes: 1}, {MaxVertices: 1}, {MaxBits: 1}} {
		_, e := Apply(context.Background(), s, nil, o)
		if !errors.Is(e, ErrLimit) {
			t.Fatalf("%+v: %v", o, e)
		}
	}
	for _, cs := range [][]FaceTransform{{{99, geom.Identity()}}, {{0, geom.Identity()}, {0, geom.Identity()}}, {{0, geom.Affine{}}}} {
		if _, e := Apply(context.Background(), s, cs, Options{}); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Apply(ctx, s, nil, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func FuzzSingleFaceVolume(f *testing.F) {
	f.Add(uint16(30))
	f.Add(uint16(0))
	f.Fuzz(func(t *testing.T, k uint16) {
		v := float64(k%64) / 16
		r, e := Shear(context.Background(), cube(t), neutral(t), []FaceShear{{3, geom.Vec3{X: v}}}, Options{})
		if e != nil {
			t.Fatal(e)
		}
		checkVolume(t, r, int64(32+k%64), 32)
	})
}
func TestNonconvexAndConversion(t *testing.T) {
	s, e := brep.Prism([]geom.Vec3{{}, {X: 2}, {X: 2, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 2}, {Y: 2}}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Apply(context.Background(), s, nil, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	_, e = Shear(context.Background(), cube(t), neutral(t), []FaceShear{{5, geom.Vec3{X: .75}}, {3, geom.Vec3{X: -.75}}, {2, geom.Vec3{Y: .75}}, {4, geom.Vec3{Y: -.75}}}, Options{MaxCoordinateError: 1e-30})
	if !errors.Is(e, ErrPrecision) {
		t.Fatal(e)
	}
}
func TestEmptyAndIdentity(t *testing.T) {
	r, e := Apply(context.Background(), brep.Solid{}, nil, Options{})
	if e != nil {
		t.Fatal(e)
	}
	checkVolume(t, r, 0, 1)
	r, e = Apply(context.Background(), cube(t), nil, Options{})
	if e != nil {
		t.Fatal(e)
	}
	checkVolume(t, r, 1, 1)
	for i, src := range r.FaceSources {
		if len(src) != 1 || r.OldToNew[src[0]] != i {
			t.Fatal(src)
		}
	}
}
