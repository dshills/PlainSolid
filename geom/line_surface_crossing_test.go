package geom

import (
	"errors"
	"math"
	"testing"
)

func TestLineSurfaceCrossingOrientation(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	reverse := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{v, u, 0} })
	line, _ := NewLine(Vec3{.3, .4, -1}, Vec3{Z: 1})
	for _, tc := range []struct {
		p    *SurfacePatch
		sign int
	}{{a, 1}, {reverse, -1}} {
		r, e := CertifyLineSurfaceCrossing(line, tc.p, SurfaceTransversalityOptions{})
		if e != nil || !r.ProvenRegular || r.Sign != tc.sign {
			t.Fatal(r, e)
		}
		hits, e := IntersectLineSplineSurface(line, tc.p, LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 2})
		if e != nil {
			t.Fatal(e)
		}
		r, e = hits.Hits[0].Crossing(line, tc.p, SurfaceTransversalityOptions{})
		if e != nil || r.Sign != tc.sign {
			t.Fatal(r, e)
		}
	}
	line, _ = NewLine(Vec3{}, Vec3{Z: -1})
	r, e := CertifyLineSurfaceCrossing(line, a, SurfaceTransversalityOptions{})
	if e != nil || r.Sign != -1 {
		t.Fatal(r, e)
	}
}
func TestLineSurfaceCrossingNearGrazingAndBoundary(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	for _, z := range []float64{math.SmallestNonzeroFloat64, 0, -math.SmallestNonzeroFloat64} {
		line, _ := NewLine(Vec3{}, Vec3{X: 1, Z: z})
		r, e := CertifyLineSurfaceCrossing(line, p, SurfaceTransversalityOptions{})
		want := 0
		if z > 0 {
			want = 1
		} else if z < 0 {
			want = -1
		}
		if e != nil || r.Sign != want {
			t.Fatal(r, e)
		}
	}
	line, _ := NewLine(Vec3{0, 0, -1}, Vec3{Z: 1})
	hits, e := IntersectLineSplineSurface(line, p, LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 2})
	if e != nil {
		t.Fatal(e)
	}
	r, e := hits.Hits[0].Crossing(line, p, SurfaceTransversalityOptions{})
	if e != nil || r.Sign != 1 {
		t.Fatal(r, e)
	}
	if _, e := CertifyLineSurfaceCrossing(line, p, SurfaceTransversalityOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
