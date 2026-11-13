package geom

import (
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestSurfacePointPlaneExactSign(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - .5} })
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	for _, tc := range []struct {
		u    float64
		sign int
	}{{0, -1}, {.5, 0}, {1, 1}, {math.Nextafter(.5, 1), 1}, {math.Nextafter(.5, 0), -1}} {
		r, e := ClassifySurfacePointPlane(p, plane, tc.u, .3, SurfacePointSignOptions{})
		if e != nil || r.Sign != tc.sign {
			t.Fatal(tc, r, e)
		}
	}
	// Rational offset is below float resolution at .5 but must retain its sign.
	u := new(big.Rat).Add(big.NewRat(1, 2), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 200)))
	r, e := ClassifySurfacePointPlaneExact(p, plane, u, big.NewRat(1, 3), SurfacePointSignOptions{})
	if e != nil || r.Sign != 1 {
		t.Fatal(r, e)
	}
}
func TestSurfacePointPlaneRationalKnotsAndOracle(t *testing.T) {
	s := quarterCylinder(t)
	s, _ = s.InsertUKnot(.3, 1)
	s, _ = s.InsertVKnot(.6, 1)
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{Z: 1}, Vec3{Z: 1})
	for _, u := range []float64{0, .3, 1} {
		for _, v := range []float64{0, .5, .6, 1} {
			r, e := ClassifySurfacePointPlane(p, plane, u, v, SurfacePointSignOptions{})
			want := 0
			if v < .5 {
				want = -1
			} else if v > .5 {
				want = 1
			}
			if e != nil || r.Sign != want {
				t.Fatal(u, v, r, e)
			}
		}
	}
	if _, e := ClassifySurfacePointPlane(p, plane, -1, 0, SurfacePointSignOptions{}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	if _, e := ClassifySurfacePointPlane(p, plane, .3, .6, SurfacePointSignOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
