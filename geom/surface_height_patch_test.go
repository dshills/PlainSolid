package geom

import (
	"errors"
	"math/big"
	"testing"
)

func TestExactSurfaceHeightDifferenceNonrepresentable(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 1} })
	tiny := 1.0 / (1 << 54)
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, tiny} })
	p, e := NewSurfaceHeightDifferencePatch(a, b, SurfaceHeightPatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane, _ := NewPlane(Vec3{Z: 1}, Vec3{Z: 1})
	r, e := ClassifySurfacePointPlane(p, plane, .3, .4, SurfacePointSignOptions{})
	if e != nil || r.Sign != -1 {
		t.Fatal(r, e)
	}
	// Float pole subtraction rounds to 1, which would falsely report boundary.
	if 1-tiny != 1 {
		t.Fatal("fixture must round")
	}
	want := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).SetFloat64(tiny))
	actual := new(big.Rat).Quo(p.net[0][2], p.net[0][3])
	if actual.Cmp(want) != 0 {
		t.Fatal(actual, want)
	}
	bounds, _ := p.Bounds()
	if bounds.Max.Z != 1 || bounds.Min.Z >= 1 {
		t.Fatal(bounds)
	}
}
func TestExactSurfaceHeightDifferenceProportionalWeights(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v} })
	source := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, .5} })
	scaled := *source
	scaled.net = make([]exactHomogeneous, len(source.net))
	for i, h := range source.net {
		for axis, v := range h {
			scaled.net[i][axis] = new(big.Rat).Mul(v, big.NewRat(3, 1))
		}
	}
	p, e := NewSurfaceHeightDifferencePatch(a, &scaled, SurfaceHeightPatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	r, e := ClassifySurfacePointPlane(p, plane, .25, .25, SurfacePointSignOptions{})
	if e != nil || r.Sign != 0 {
		t.Fatal(r, e)
	}
	if _, e = NewSurfaceHeightDifferencePatch(a, source, SurfaceHeightPatchOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	mismatch := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{2 * u, v, 0} })
	if _, e = NewSurfaceHeightDifferencePatch(a, mismatch, SurfaceHeightPatchOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
