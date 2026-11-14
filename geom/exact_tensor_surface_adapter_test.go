package geom

import (
	"context"
	"testing"
)

func TestExactTensorSurfaceAdapter(t *testing.T) {
	ctx := context.Background()
	p := affineTriangleSupport(t, true, 0)
	s, e := NewExactTensorSurface(ctx, p, SurfacePatchOptions{})
	if e != nil || s.ExactPatch() != p {
		t.Fatal(e)
	}
	for _, uv := range [][2]float64{{.25, .5}, {.3, .7}, {0, 1}} {
		a, e := s.Eval(uv[0], uv[1])
		if e != nil {
			t.Fatal(e)
		}
		b, e := p.Eval(uv[0], uv[1])
		if e != nil || a != b {
			t.Fatal("evaluation")
		}
		au, av, e := s.Partials(uv[0], uv[1])
		if e != nil {
			t.Fatal(e)
		}
		bu, bv, e := p.Partials(uv[0], uv[1])
		if e != nil || au != bu || av != bv {
			t.Fatal("derivatives")
		}
	}
	for _, o := range []SurfacePatchOptions{{MaxPoles: 1}, {MaxOperations: s.ExactOperations() - 1}} {
		if got, e := NewExactTensorSurface(ctx, p, o); e == nil || got != nil {
			t.Fatal("bounded adapter")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := NewExactTensorSurface(c, p, SurfacePatchOptions{}); e == nil || got != nil {
		t.Fatal("cancel")
	}
}
