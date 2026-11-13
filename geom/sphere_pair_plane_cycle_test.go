package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSpherePairPlaneCycleOrientation(t *testing.T) {
	for _, n := range []Vec3{{Y: 1, Z: 1}, {Y: -1, Z: -1}, {Y: 1}, {Y: -1}} {
		for _, op := range []SphereSetOperation{SphereSetUnion, SphereSetIntersection, SphereSetDifference} {
			s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, op)
			r, e := s.PlaneBoundaryCycle(context.Background(), Vec3{}, n, SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(n, op, e)
			}
			if r.Arcs[0].EndContact != r.Arcs[1].StartContact || r.Arcs[1].EndContact != r.Arcs[0].StartContact {
				t.Fatal("cycle")
			}
			for i, a := range r.Arcs {
				inside := op == SphereSetIntersection || (op == SphereSetDifference && i == 1)
				// Exact extrema on common X direction lie on each support circle.
				x := int64(-5)
				if i == 1 {
					x = 7
				}
				if inside {
					if i == 0 {
						x = 5
					} else {
						x = 1
					}
				}
				val := new(big.Rat).Add(a.Offset, new(big.Rat).Mul(a.Covector[0], big.NewRat(x, 1)))
				if val.Sign()*a.InteriorSign <= 0 {
					t.Fatal("arc selector", a, x)
				}
				dx := new(big.Rat).Sub(big.NewRat(x, 1), a.Center[0])
				sq := new(big.Rat).Mul(r.Chart.Gram[0][0], new(big.Rat).Mul(dx, dx))
				if sq.Cmp(a.RadiusSquared) != 0 {
					t.Fatal("Gram circle")
				}
				// In UV, the right-hand arc of either ellipse traverses bottom to top
				// when CCW, independent of the raw plane normal or its scaling.
				right := (i == 0 && inside) || (i == 1 && !inside)
				wantStartY := -1
				if !right {
					wantStartY = 1
				}
				if !a.CCW {
					wantStartY = -wantStartY
				}
				p := r.Contacts[a.StartContact]
				actual := p.Sign * p.UVVector[1].Sign()
				if actual != wantStartY {
					t.Fatal("directed endpoint", n, op, i, actual, wantStartY)
				}
				if a.CCW != (op != SphereSetDifference || i != 1) {
					t.Fatal("material-left")
				}
				for _, p := range r.Contacts {
					c := new(big.Rat).Add(a.Offset, new(big.Rat).Add(new(big.Rat).Mul(a.Covector[0], p.UVBase[0]), new(big.Rat).Mul(a.Covector[1], p.UVBase[1])))
					v := new(big.Rat).Add(new(big.Rat).Mul(a.Covector[0], p.UVVector[0]), new(big.Rat).Mul(a.Covector[1], p.UVVector[1]))
					if c.Sign() != 0 || v.Sign() != 0 {
						t.Fatal("exact radical axis")
					}
				}
			}
		}
	}
}
func TestSpherePairPlaneCycleGuards(t *testing.T) {
	for _, x := range []float64{0, 2, 8, 20} {
		s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: x}, 3, SphereSetUnion)
		r, e := s.PlaneBoundaryCycle(context.Background(), Vec3{}, Vec3{Y: 1, Z: 1}, SurfacePatchEvaluationOptions{})
		if e != ErrDomain || r.Chart.NormalSquared != nil {
			t.Fatal(x, r, e)
		}
	}
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, SphereSetUnion)
	r, e := s.PlaneBoundaryCycle(context.Background(), Vec3{}, Vec3{Y: 1, Z: 1}, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || r.Chart.NormalSquared != nil {
		t.Fatal("atomic", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := s.PlaneBoundaryCycle(ctx, Vec3{}, Vec3{Z: 1}, SurfacePatchEvaluationOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
}
