package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestRawPlaneCubicRationalRecovery(t *testing.T) {
	tests := []struct {
		z     []float64
		roots []string
		mult  int
	}{
		{[]float64{-3, 0, 2, 12}, []string{"1/3"}, 1},                  // 3(3t-1)(t²+1)
		{[]float64{-24, 22, -13, 6}, []string{"1/3", "2/3", "4/5"}, 1}, // 3(3t-1)(3t-2)(5t-4)
		{[]float64{-1, 2, -4, 8}, []string{"1/3"}, 3},                  // (3t-1)³
		{[]float64{0, 2, -5, 6}, []string{"0", "1/3", "2/3"}, 1},       // 3t(3t-1)(3t-2)
		{[]float64{-6, 5, -2, 0}, []string{"1/3", "2/3", "1"}, 1},      // 3(t-1)(3t-1)(3t-2)
		{[]float64{-3, 65534, 131070, 393216}, []string{"1/65537"}, 1},
	}
	for _, tc := range tests {
		var poles []Vec3
		for i, z := range tc.z {
			poles = append(poles, Vec3{X: float64(i), Z: z})
		}
		s, _ := NewBezier(poles, nil)
		curve := isoIntersectionFixture(t, s)
		result, e := curve.IntersectRawPlane(context.Background(), Vec3{}, Vec3{X: 1}, Vec3{Y: 1}, SplinePlaneOptions{})
		if e != nil || !result.Complete || len(result.Hits) != len(tc.roots) {
			t.Fatal(result, e)
		}
		for i, h := range result.Hits {
			if h.Parameter == nil || h.Parameter.RatString() != tc.roots[i] || h.Point[2].Sign() != 0 || h.Multiplicity != tc.mult {
				t.Fatal(tc.roots, h)
			}
		}
	}
}
func TestRawPlaneCubicRecoveryRejectsNearRational(t *testing.T) {
	s, _ := NewBezier([]Vec3{{Z: -3}, {X: 1}, {X: 2, Z: 2}, {X: 3, Z: 12}}, nil)
	curve := isoIntersectionFixture(t, s)
	shifted, e := curve.Transform(Translation(Vec3{Z: math.Ldexp(1, -70)}), SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	result, e := shifted.IntersectRawPlane(context.Background(), Vec3{}, Vec3{X: 1}, Vec3{Y: 1}, SplinePlaneOptions{})
	if e != nil || !result.Complete || len(result.Hits) != 1 {
		t.Fatal(result, e)
	}
	if result.Hits[0].Parameter != nil && result.Hits[0].Parameter.Cmp(big.NewRat(1, 3)) == 0 {
		t.Fatal("rounded near root became exact")
	}
	irrational, _ := NewBezier([]Vec3{{Z: -.5}, {Z: -.5}, {Z: -.5}, {Z: .5}}, nil)
	result, e = isoIntersectionFixture(t, irrational).IntersectRawPlane(context.Background(), Vec3{}, Vec3{X: 1}, Vec3{Y: 1}, SplinePlaneOptions{})
	if e != nil || len(result.Hits) != 1 || result.Hits[0].Parameter != nil {
		t.Fatal(result, e)
	}
	b := &polynomialBudget{maxOperations: 1, maxBits: 16384}
	_, e = intervalRationalConvergents(big.NewRat(100, 301), big.NewRat(101, 302), b)
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}

func TestPlaneArcCubicEndpointRecovery(t *testing.T) {
	var poles []Vec3
	for i, z := range []float64{-3, 0, 2, 12} {
		for _, v := range []float64{0, 1} {
			poles = append(poles, Vec3{float64(i), v, z})
		}
	}
	s, _ := NewBSplineSurface(3, 1, 4, 2, poles, []float64{0, 0, 0, 0, 1, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	patch, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	arcs, e := CertifySurfacePlaneArcs(patch, plane, SurfacePlaneArcOptions{})
	if e != nil || len(arcs.Arcs) != 1 {
		t.Fatal(arcs, e)
	}
	for end := 0; end < 2; end++ {
		point, e := arcs.Arcs[0].ExactEndpoint(end, ArcExactEndpointOptions{})
		if e != nil || point.Parameters[0].Cmp(big.NewRat(1, 3)) != 0 || point.Point[0].Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal(point, e)
		}
	}
}
