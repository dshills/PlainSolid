package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestRationalBiquadraticCorrespondenceIndependent(t *testing.T) {
	ctx := context.Background()
	var poles []geom.Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: float64(i*j) / 4})
		}
	}
	weights := []float64{1, .25, 2, .5, 3, 1.25, 2, .75, 1}
	knots := []float64{0, 0, 0, 1, 1, 1}
	surface, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, weights)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	uv := [3]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}}
	model := [3]geom.Vec3{poles[0], poles[6], poles[8]}
	for i := range model {
		model[i].Z += .0625
	}
	bound, e := BoundSplineTriangleDeviation(ctx, patch, uv, model)
	if e != nil || !bound.BothDirections {
		t.Fatal(bound, e)
	}
	bernstein := func(x *big.Rat) [3]*big.Rat {
		q := new(big.Rat).Sub(big.NewRat(1, 1), x)
		return [3]*big.Rat{new(big.Rat).Mul(q, q), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(q, x)), new(big.Rat).Mul(x, x)}
	}
	for i := 0; i <= 12; i++ {
		for j := 0; j <= 12-i; j++ {
			s, t0 := big.NewRat(int64(i), 12), big.NewRat(int64(j), 12)
			u, v := new(big.Rat).Add(s, t0), t0
			bu, bv := bernstein(u), bernstein(v)
			den := new(big.Rat)
			num := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
			for a := 0; a < 3; a++ {
				for b := 0; b < 3; b++ {
					index := a*3 + b
					weight := new(big.Rat).Mul(new(big.Rat).Mul(bu[a], bv[b]), ratInset(weights[index]))
					den.Add(den, weight)
					for k, value := range []float64{poles[index].X, poles[index].Y, poles[index].Z} {
						num[k].Add(num[k], new(big.Rat).Mul(weight, ratInset(value)))
					}
				}
			}
			lambda := [3]*big.Rat{new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Add(s, t0)), s, t0}
			error := new(big.Rat)
			for k := 0; k < 3; k++ {
				actual := new(big.Rat).Quo(num[k], den)
				linear := new(big.Rat)
				for index := 0; index < 3; index++ {
					p := []float64{model[index].X, model[index].Y, model[index].Z}
					linear.Add(linear, new(big.Rat).Mul(lambda[index], ratInset(p[k])))
				}
				d := new(big.Rat).Sub(actual, linear)
				error.Add(error, new(big.Rat).Mul(d, d))
			}
			if error.Cmp(bound.SquaredDistance) > 0 {
				t.Fatal("false homogeneous residual bound", i, j, error, bound)
			}
		}
	}
	scaled := append([]float64(nil), weights...)
	for i := range scaled {
		scaled[i] *= 8
	}
	equal, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, scaled)
	if e != nil {
		t.Fatal(e)
	}
	ep, e := geom.NewSurfacePatch(equal, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	same, e := BoundSplineTriangleDeviation(ctx, ep, uv, model)
	if e != nil || same.SquaredDistance.Cmp(bound.SquaredDistance) != 0 {
		t.Fatal("homogeneous scaling changed exact certificate", e)
	}

	r, e := TessellatePatchCertified(ctx, patch, CertifiedPatchMeshOptions{Deflection: .2, MaxSubdivisions: 32})
	if e != nil || !r.BothDirections {
		t.Fatal("rational certified mesh", r.Subdivisions, e)
	}
}
