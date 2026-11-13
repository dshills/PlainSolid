package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func endpointEdgeTubeFixture(t *testing.T) (VerifiedSurfaceTube, *SurfacePatch, *SurfacePatch) {
	t.Helper()
	sa, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{0, 0, 0}, {0, 3, 0}, {1, 0, 0}, {1, 3, 0}}, []float64{0, 0, 1, 1}, []float64{0, 0, 3, 3}, nil)
	a, _ := NewSurfacePatch(sa, SurfacePatchOptions{})
	a, _ = a.Trim(ParameterRectangle{UMin: 0, UMax: .2, VMin: .9, VMax: 1.1})
	sb, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{-1, 1, -1}, {-1, 1, 2}, {1, 1, -1}, {1, 1, 2}}, []float64{-1, -1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	b, _ := NewSurfacePatch(sb, SurfacePatchOptions{})
	b, _ = b.Trim(ParameterRectangle{UMin: -.1, UMax: .3, VMin: .25, VMax: .4})
	tube, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return tube, a, b
}
func TestTubeEndpointEdgeExactRationalAndOriginalFace(t *testing.T) {
	tube, a, b := endpointEdgeTubeFixture(t)
	face := ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 3}
	hint := [4]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(1, 3)}
	r, e := tube.CertifyEndpointOnEdge(context.Background(), false, 0, face, SurfaceEdgeUMin, hint, TubeEndpointEdgeOptions{})
	if e != nil || !r.Proven || r.NormalizedEdgeParameter.Cmp(big.NewRat(1, 3)) != 0 || r.EdgeParameter.Cmp(big.NewRat(1, 1)) != 0 || r.Point[0].Sign() != 0 || r.Point[1].Cmp(big.NewRat(1, 1)) != 0 || r.Point[2].Sign() != 0 {
		t.Fatal(r, e)
	}
	upperFace := face
	upperFace.UMax = .2
	upperHint := [4]*big.Rat{new(big.Rat).SetFloat64(.2), big.NewRat(1, 1), new(big.Rat).SetFloat64(.2), big.NewRat(1, 3)}
	upperProof, e := tube.CertifyEndpointOnEdge(context.Background(), true, 0, upperFace, SurfaceEdgeUMax, upperHint, TubeEndpointEdgeOptions{})
	if e != nil || !upperProof.Proven || upperProof.NormalizedEdgeParameter.Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(upperProof, e)
	}
	if !r.PointBounds.Contains(Vec3{0, 1, 0}) {
		t.Fatal(r)
	}
	hint[1].SetInt64(2)
	if r.Parameters[1].Cmp(big.NewRat(1, 1)) != 0 || r.EdgeParameter.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("input alias")
	}
	r.Point[0].SetInt64(9)
	hint[1] = big.NewRat(1, 1)
	again, e := tube.CertifyEndpointOnEdge(context.Background(), false, 0, face, SurfaceEdgeUMin, hint, TubeEndpointEdgeOptions{})
	if e != nil || again.Point[0].Sign() != 0 {
		t.Fatal(again, e)
	}
	swapped, e := NewVerifiedSurfaceTube(b, a, SurfacePairTubeOptions{FreeAxis: 2})
	if e != nil {
		t.Fatal(e)
	}
	swappedHint := [4]*big.Rat{hint[2], hint[3], hint[0], hint[1]}
	q, e := swapped.CertifyEndpointOnEdge(context.Background(), false, 1, face, SurfaceEdgeUMin, swappedHint, TubeEndpointEdgeOptions{})
	if e != nil || !q.Proven || q.NormalizedEdgeParameter.Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(q, e)
	}
}
func TestTubeEndpointEdgeRejectsAndLimits(t *testing.T) {
	tube, _, _ := endpointEdgeTubeFixture(t)
	face := ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 3}
	hint := [4]*big.Rat{big.NewRat(0, 1), big.NewRat(1001, 1000), big.NewRat(0, 1), big.NewRat(1, 3)}
	r, e := tube.CertifyEndpointOnEdge(context.Background(), false, 0, face, SurfaceEdgeUMin, hint, TubeEndpointEdgeOptions{})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
	hint[1] = big.NewRat(1, 1)
	_, e = tube.CertifyEndpointOnEdge(context.Background(), false, 0, face, SurfaceEdgeVMin, hint, TubeEndpointEdgeOptions{})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	_, e = tube.CertifyEndpointOnEdge(context.Background(), true, 0, face, SurfaceEdgeUMax, hint, TubeEndpointEdgeOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	invalid := face
	invalid.VMax = 1
	_, e = tube.CertifyEndpointOnEdge(context.Background(), false, 0, invalid, SurfaceEdgeUMin, hint, TubeEndpointEdgeOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	_, e = tube.CertifyEndpointOnEdge(context.Background(), false, 0, face, SurfaceEdgeUMin, hint, TubeEndpointEdgeOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	r, e = tube.CertifyEndpointOnEdge(distanceCancelContext(2), false, 0, face, SurfaceEdgeUMin, hint, TubeEndpointEdgeOptions{})
	if !errors.Is(e, context.Canceled) || r.Proven {
		t.Fatal(r, e)
	}
}
