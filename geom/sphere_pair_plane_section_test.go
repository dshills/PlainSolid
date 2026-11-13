package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSpherePairPlaneSectionRadical(t *testing.T) {
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, SphereSetIntersection)
	r, e := s.PlaneSection(context.Background(), Vec3{}, Vec3{Y: 1, Z: 1}, SurfacePatchEvaluationOptions{})
	if e != nil || len(r.Contacts) != 2 {
		t.Fatal(r, e)
	}
	for _, p := range r.Contacts {
		if p.Location != BoreBoundary || p.Boundary != 3 || p.Incidence != 3 {
			t.Fatal(p)
		}
		for i := 0; i < 2; i++ {
			constant, cross := new(big.Rat).Neg(s.radius2[i]), new(big.Rat)
			for j := 0; j < 3; j++ {
				d := new(big.Rat).Sub(p.Base[j], s.center[i][j])
				constant.Add(constant, new(big.Rat).Mul(d, d))
				constant.Add(constant, new(big.Rat).Mul(p.Radicand, new(big.Rat).Mul(p.Vector[j], p.Vector[j])))
				cross.Add(cross, new(big.Rat).Mul(d, p.Vector[j]))
			}
			if constant.Sign() != 0 || cross.Sign() != 0 {
				t.Fatal("sphere radical identity")
			}
		}
		for j := 0; j < 3; j++ {
			base, vec := new(big.Rat).Set(r.Chart.Origin[j]), new(big.Rat)
			for k := 0; k < 2; k++ {
				base.Add(base, new(big.Rat).Mul(r.Chart.Basis[k][j], p.UVBase[k]))
				vec.Add(vec, new(big.Rat).Mul(r.Chart.Basis[k][j], p.UVVector[k]))
			}
			if base.Cmp(p.Base[j]) != 0 || vec.Cmp(p.Vector[j]) != 0 {
				t.Fatal("UV identity")
			}
		}
		// x=4 and y=-z=+-sqrt(9/2), independently known.
		if p.Base[0].Cmp(big.NewRat(4, 1)) != 0 || new(big.Rat).Mul(p.Radicand, new(big.Rat).Mul(p.Vector[1], p.Vector[1])).Cmp(big.NewRat(9, 2)) != 0 {
			t.Fatal(p)
		}
	}
}
func TestSpherePairPlaneSectionRegimes(t *testing.T) {
	for _, tc := range []struct {
		x   float64
		n   int
		op  SphereSetOperation
		loc BorePointLocation
	}{{20, 0, SphereSetUnion, BoreOutside}, {8, 1, SphereSetUnion, BoreBoundary}, {8, 1, SphereSetIntersection, BoreOutside}} {
		s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: tc.x}, 3, tc.op)
		r, e := s.PlaneSection(context.Background(), Vec3{}, Vec3{Y: 2, Z: 3}, SurfacePatchEvaluationOptions{})
		if e != nil || len(r.Contacts) != tc.n {
			t.Fatal(tc, r, e)
		}
		if tc.n > 0 && (r.Contacts[0].Sign != 0 || r.Contacts[0].Location != tc.loc) {
			t.Fatal(r)
		}
	}
	// Equal projected circles on a plane between distinct sphere centers.
	s, _ := NewSpherePairMaterial(Vec3{Z: -2}, 3, Vec3{Z: 2}, 3, SphereSetDifference)
	r, e := s.PlaneSection(context.Background(), Vec3{}, Vec3{Z: 3}, SurfacePatchEvaluationOptions{})
	if e != nil || !r.CoincidentCircle || len(r.Contacts) != 0 {
		t.Fatal(r, e)
	}
	// Identical removed spheres have no material, even on their common circle.
	s, _ = NewSpherePairMaterial(Vec3{}, 3, Vec3{}, 3, SphereSetDifference)
	r, e = s.PlaneSection(context.Background(), Vec3{}, Vec3{X: 1, Y: 2, Z: 3}, SurfacePatchEvaluationOptions{})
	if e != nil || !r.CoincidentCircle || r.CoincidentLocation != BoreOutside {
		t.Fatal(r, e)
	}
	// One support pole, retained as an original support event inside other sphere.
	s, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{}, 3, SphereSetUnion)
	r, e = s.PlaneSection(context.Background(), Vec3{Z: 3}, Vec3{Z: 2}, SurfacePatchEvaluationOptions{})
	if e != nil || len(r.Contacts) != 1 || r.Contacts[0].Location != BoreInside || r.Contacts[0].Incidence != SpherePairSecond {
		t.Fatal(r, e)
	}
	r, e = s.PlaneSection(context.Background(), Vec3{}, Vec3{X: 1, Y: 2, Z: 3}, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || r.Chart.NormalSquared != nil {
		t.Fatal("atomic", r, e)
	}
}
