package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestCurveSurfaceRationalProposal(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	s, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	curve, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(1, 3), big.NewRat(2, 3), big.NewRat(-1, 3), big.NewRat(1, 1)}, {big.NewRat(1, 3), big.NewRat(2, 3), big.NewRat(2, 3), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	hit := CurveSurfaceHit{SurfaceCell: p.Domain(), CurveCell: ParameterSpan{Start: 0, End: 1}, UV: UVBox{Min: Vec2{}, Max: Vec2{X: 1, Y: 1}}, CurveParameter: CertifiedScalarRange{Min: 0, Max: 1}, Direction: -1}
	q := big.NewRat(1, 3)
	r, e := CertifyCurveSurfaceRationalHit(ctx, curve, p, hit, q, q, big.NewRat(2, 3), SurfacePatchEvaluationOptions{})
	if e != nil || !r.Contact || !r.Regular || r.Direction != 1 || r.Point[0].Cmp(q) != 0 {
		t.Fatal(r, e)
	}
	r.Point[0].SetInt64(99)
	wrong := new(big.Rat).Add(q, new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
	r, e = CertifyCurveSurfaceRationalHit(ctx, curve, p, hit, wrong, q, big.NewRat(2, 3), SurfacePatchEvaluationOptions{})
	if e != nil || r.Contact || r.Regular {
		t.Fatal(r, e)
	}
	tangent, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(0, 1), big.NewRat(2, 3), big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(2, 3), big.NewRat(0, 1), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = CertifyCurveSurfaceRationalHit(ctx, tangent, p, hit, q, q, big.NewRat(2, 3), SurfacePatchEvaluationOptions{})
	if e != nil || !r.Contact || r.Regular || r.Direction != 0 {
		t.Fatal(r, e)
	}

	hit.CurveParameter.Max = .25
	if _, e = CertifyCurveSurfaceRationalHit(ctx, curve, p, hit, q, q, big.NewRat(2, 3), SurfacePatchEvaluationOptions{}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	hit.CurveParameter.Max = 1
	if _, e = CertifyCurveSurfaceRationalHit(ctx, curve, p, hit, q, q, big.NewRat(2, 3), SurfacePatchEvaluationOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = CertifyCurveSurfaceRationalHit(cancelCtx, curve, p, hit, q, q, big.NewRat(2, 3), SurfacePatchEvaluationOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
