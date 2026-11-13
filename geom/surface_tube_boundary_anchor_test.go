package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestBoundaryAnchoredSurfaceTubeTwoChartSides(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	// A=(u,v,uv) on x>=0, B=(s,t,s-t) on y>=0.
	// The branch has s=u, v=t=u/(1+u) and starts at both minima.
	makePatch := func(p []Vec3) *SurfacePatch {
		t.Helper()
		s, e := NewBSplineSurface(1, 1, 2, 2, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		q, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return q
	}
	a := makePatch([]Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}})
	b := makePatch([]Vec3{{}, {Y: 1, Z: -1}, {X: 1, Z: 1}, {X: 1, Y: 1}})
	a, _ = a.Trim(ParameterRectangle{UMax: .125, VMax: .25})
	// Reverse the B chart coordinates so the second boundary is t=0,
	// and make s=0 interior by extending its original affine support.
	ks := []float64{-.25, -.25, .25, .25}
	bs, e := NewBSplineSurface(1, 1, 2, 2,
		[]Vec3{{X: -.25, Z: -.25}, {X: -.25, Y: 1, Z: -1.25}, {X: .25, Z: .25}, {X: .25, Y: 1, Z: -.75}}, ks, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e = NewSurfacePatch(bs, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Trim(ParameterRectangle{UMin: -.25, UMax: .25, VMax: .25})
	if e != nil {
		t.Fatal(e)
	}
	standard, e := CertifySurfacePairTube(a, b, SurfacePairTubeOptions{})
	if e != nil || standard.ProvenBranch {
		t.Fatal("strict interior tube accepted a dependent boundary", standard, e)
	}
	var opts SurfaceTubeBoundaryOptions
	for i := range opts.Anchor {
		opts.Anchor[i] = new(big.Rat)
	}
	tube, e := NewBoundaryAnchoredSurfaceTube(ctx, a, b, opts)
	if e != nil {
		t.Fatal(e)
	}
	start, e := tube.Evaluate(SurfaceTubeEvaluationOptions{})
	if e != nil || !start.Complete || start.Point != (Vec3{}) || start.ParametersB.Min != start.ParametersB.Max {
		t.Fatal(start, e)
	}
	exact, e := tube.CertifyEndpoint(ctx, false, opts.Anchor, TubeEndpointOptions{})
	if e != nil || !exact.Proven {
		t.Fatal(exact, e)
	}
	for _, u := range []float64{1.0 / 32, 1.0 / 16, 1.0 / 8} {
		q, e := tube.Evaluate(SurfaceTubeEvaluationOptions{Parameter: u})
		v := u / (1 + u)
		if e != nil || !q.Complete || q.ParametersA.Min.Y > v || q.ParametersA.Max.Y < v ||
			q.ParametersB.Min.X > u || q.ParametersB.Max.X < u {
			t.Fatal(u, q, e)
		}
	}
	opts.Anchor[3].SetInt64(1)
	again, e := tube.Evaluate(SurfaceTubeEvaluationOptions{})
	if e != nil || again.ParametersB.Max.Y != 0 {
		t.Fatal("proof alias", again, e)
	}
	if _, e := NewBoundaryAnchoredSurfaceTube(ctx, a, b, opts); !errors.Is(e, ErrDomain) {
		t.Fatal("accepted invalid anchor", e)
	}
	opts.Anchor[3].SetInt64(0)
	narrow, _ := b.Trim(ParameterRectangle{UMin: -.25, UMax: .25, VMax: .0625})
	if _, e := NewBoundaryAnchoredSurfaceTube(ctx, a, narrow, opts); !errors.Is(e, ErrConvergence) {
		t.Fatal("accepted branch exiting dependent side", e)
	}
}
