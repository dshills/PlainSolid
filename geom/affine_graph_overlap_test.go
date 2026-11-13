package geom

import (
	"context"
	"errors"
	"testing"
)

func TestClipAffineGraphOverlap(t *testing.T) {
	makePatch := func(offset, scale float64, swap bool) *SurfacePatch {
		poles := make([]Vec3, 4)
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				u, v := float64(i), float64(j)
				if swap {
					u, v = 1-v, u
				}
				poles[2*i+j] = Vec3{X: offset + scale*u, Y: v, Z: float64(i * j)}
			}
		}
		k := []float64{0, 0, 1, 1}
		s, e := NewBSplineSurface(1, 1, 2, 2, poles, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	ctx := context.Background()
	a := makePatch(0, 1, false)
	r, e := ClipAffineXYGraphOverlap(ctx, a, makePatch(.5, 1, true), SurfacePatchOptions{})
	if e != nil || r.Disjoint || r.A != (ParameterRectangle{UMin: .5, UMax: 1, VMin: 0, VMax: 1}) || r.B != (ParameterRectangle{UMin: 0, UMax: 1, VMin: .5, VMax: 1}) {
		t.Fatal(r, e)
	}
	r, e = ClipAffineXYGraphOverlap(ctx, a, makePatch(2, 1, false), SurfacePatchOptions{})
	if e != nil || !r.Disjoint {
		t.Fatal(r, e)
	}
	_, e = ClipAffineXYGraphOverlap(ctx, a, makePatch(1, 1, false), SurfacePatchOptions{})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal("touch", e)
	}
	_, e = ClipAffineXYGraphOverlap(ctx, a, makePatch(0, 3, false), SurfacePatchOptions{})
	if !errors.Is(e, ErrUnrepresentable) {
		t.Fatal("nonbinary trim", e)
	}
	_, e = ClipAffineXYGraphOverlap(ctx, a, makePatch(.5, 1, false), SurfacePatchOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal("limit", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = ClipAffineXYGraphOverlap(canceled, a, a, SurfacePatchOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
