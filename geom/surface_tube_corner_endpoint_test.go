package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestTubeCornerEndpoint(t *testing.T) {
	ctx := context.Background()
	build := func(p []Vec3, ku, kv []float64) *SurfacePatch {
		s, e := NewBSplineSurface(1, 1, 2, 2, p, ku, kv, nil)
		if e != nil {
			t.Fatal(e)
		}
		r, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	k := []float64{0, 0, 1, 1}
	wide := []float64{-1, -1, 2, 2}
	a := build([]Vec3{{}, {Z: 1}, {Y: 1}, {Y: 1, Z: 1}}, k, k)
	b := build([]Vec3{{X: -1, Y: -1, Z: -.5}, {X: -1, Y: 2, Z: .25}, {X: 2, Y: -1, Z: .25}, {X: 2, Y: 2, Z: 1}}, wide, wide)
	domains := [2]ParameterRectangle{a.Domain(), b.Domain()}
	a, _ = a.Trim(ParameterRectangle{UMax: .125, VMax: .125})
	b, _ = b.Trim(ParameterRectangle{UMin: -.125, UMax: .125, VMin: 0, VMax: .25})
	hints := [4]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}
	strict, e := CertifySurfacePairTube(a, b, SurfacePairTubeOptions{})
	if e != nil || strict.ProvenBranch {
		t.Fatal("generic interior gate changed", e)
	}
	tube, e := NewBoundaryAnchoredSurfaceTube(ctx, a, b, SurfaceTubeBoundaryOptions{Anchor: hints})
	if e != nil {
		t.Fatal(e)
	}
	p, e := tube.CertifyCornerEndpoint(ctx, false, 0, domains, TubeEndpointOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if p.Sides() != ([2]SurfaceEdge{SurfaceEdgeUMin, SurfaceEdgeVMin}) || p.Chart() != 0 || p.Domains() != domains {
		t.Fatal("ownership")
	}
	if !p.Matches(tube) || p.Matches(VerifiedSurfaceTube{}) {
		t.Fatal("source binding")
	}
	other, e := NewBoundaryAnchoredSurfaceTube(ctx, a, b, SurfaceTubeBoundaryOptions{Anchor: hints})
	if e != nil {
		t.Fatal(e)
	}
	if p.Matches(other) {
		t.Fatal("independent proof rebound")
	}
	for _, v := range p.Point() {
		if v.Sign() != 0 {
			t.Fatal(v)
		}
	}
	q := p.Parameters()
	q[0].SetInt64(1)
	if p.Parameters()[0].Sign() != 0 {
		t.Fatal("alias")
	}
	for _, u := range []float64{.03125, .0625, .125} {
		r, e := tube.Evaluate(SurfaceTubeEvaluationOptions{Parameter: u})
		if e != nil || !r.Complete || !r.Bounds.Contains(Vec3{Y: u, Z: u / 4}) {
			t.Fatal("continuation", r, e)
		}
	}
	for _, bad := range []struct {
		upper   bool
		chart   int
		domains [2]ParameterRectangle
	}{{true, 0, domains}, {false, 1, domains}, {false, 0, [2]ParameterRectangle{domains[0], {UMin: 0, UMax: 2, VMin: 0, VMax: 2}}}, {false, 0, [2]ParameterRectangle{{UMin: -1, UMax: 1, VMin: 0, VMax: 1}, domains[1]}}} {
		if r, e := tube.CertifyCornerEndpoint(ctx, bad.upper, bad.chart, bad.domains, TubeEndpointOptions{}); e == nil || r != nil {
			t.Fatal("bad ownership")
		}
	}
	if r, e := tube.CertifyCornerEndpoint(ctx, false, 0, domains, TubeEndpointOptions{MaxOperations: 1}); e == nil || r != nil {
		t.Fatal("budget")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = tube.CertifyCornerEndpoint(canceled, false, 0, domains, TubeEndpointOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
