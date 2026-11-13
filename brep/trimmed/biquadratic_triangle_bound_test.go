package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestBiquadraticTriangleCorrespondenceIndependent(t *testing.T) {
	ctx := context.Background()
	var poles []geom.Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := 0.
			if i == 2 && j == 2 {
				z = 1
			}
			poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
		}
	}
	k := []float64{0, 0, 0, 1, 1, 1}
	surface, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	uv := [3]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}}
	model := [3]geom.Vec3{{Z: .0625}, {X: 1, Z: .0625}, {X: 1, Y: 1, Z: 1.0625}}
	bound, e := BoundSplineTriangleDeviation(ctx, p, uv, model)
	if e != nil || !bound.BothDirections {
		t.Fatal(bound, e)
	}
	// Exact independent correspondence: S(u,v)=(u,v,u²v²), target
	// interpolation=(u,v,lambda2+1/16), u=lambda1+lambda2,v=lambda2.
	for i := 0; i <= 20; i++ {
		for j := 0; j <= 20-i; j++ {
			s, t0 := big.NewRat(int64(i), 20), big.NewRat(int64(j), 20)
			u := new(big.Rat).Add(s, t0)
			z := new(big.Rat).Mul(new(big.Rat).Mul(u, u), new(big.Rat).Mul(t0, t0))
			residual := new(big.Rat).Sub(z, new(big.Rat).Add(t0, big.NewRat(1, 16)))
			error := new(big.Rat).Mul(residual, residual)
			if error.Cmp(bound.SquaredDistance) > 0 {
				t.Fatal("false exact correspondence bound", i, j, error, bound)
			}
		}
	}
	r, e := TessellatePatchCertified(ctx, p, CertifiedPatchMeshOptions{Deflection: .05, MaxSubdivisions: 16})
	if e != nil || !r.BothDirections || r.Subdivisions > 8 {
		t.Fatal("practical biquadratic refinement", r.Subdivisions, e)
	}
	// Unequal rational weights must not inherit a polynomial certificate.
	weights := []float64{1, 1, 1, 1, 2, 1, 1, 1, 1}
	rational, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, weights)
	if e != nil {
		t.Fatal(e)
	}
	rp, e := geom.NewSurfacePatch(rational, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := rp.BiquadraticCoefficients(); ok {
		t.Fatal("rational patch falsely polynomial")
	}
}
