package geom

import (
	"context"
	"errors"
	"testing"
)

func TestSurfaceTubeDerivativeSigns(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	a, _ = a.Trim(ParameterRectangle{.2, .8, 0, 1})
	for _, reverse := range []bool{false, true} {
		b := arcTestPatch(t, func(u, v float64) Vec3 {
			if reverse {
				u = 1 - u
			}
			return Vec3{u, .4, v - .3}
		})
		tube, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{FreeAxis: 0})
		if e != nil {
			t.Fatal(e)
		}
		r, e := tube.ParameterDerivativeSigns(ctx, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := [4]int{1, 0, 1, 0}
		if reverse {
			want[2] = -1
		}
		for i := range want {
			if !r.Proven[i] || r.Sign[i] != want[i] || !r.HasRange[i] {
				t.Fatal(r, want)
			}
		}
		_, e = tube.ParameterDerivativeSigns(ctx, SurfacePatchOptions{MaxOperations: 1})
		if !errors.Is(e, ErrConvergence) {
			t.Fatal(e)
		}
		c, cancel := context.WithCancel(ctx)
		cancel()
		_, e = tube.ParameterDerivativeSigns(c, SurfacePatchOptions{})
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	}
}
