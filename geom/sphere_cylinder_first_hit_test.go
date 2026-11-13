package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSphereCylinderFirstDifferenceHit(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	s, e := NewOffAxisSphereFiniteCylinderDifference(5, 3, Vec2{X: 1}, -2, 2)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.FirstBoundaryHit(ctx, Vec3{X: 1}, Vec3{Z: 1}, 6, o)
	if e != nil || !r.HasHit || r.Boundary != BoreUpperCap || r.Bounds.Lower.Cmp(big.NewRat(2, 1)) != 0 || len(r.Normals) != 1 || r.Normals[0].Constant[2].Sign() != -1 || r.Normals[0].DirectionSign != -1 || r.Tangent || r.Isolated {
		t.Fatal("enter retained material at cutter cap", r, e)
	}
	r, e = s.FirstBoundaryHit(ctx, Vec3{X: 1, Z: 3}, Vec3{Z: -2}, 3, o)
	if e != nil || !r.HasHit || r.Bounds.Lower.Cmp(big.NewRat(1, 2)) != 0 || r.Normals[0].DirectionSign != 1 {
		t.Fatal("raw direction units", r, e)
	}
	r, e = s.FirstBoundaryHit(ctx, Vec3{X: -4}, Vec3{X: 1}, 4, o)
	if e != nil || !r.HasHit || r.Boundary != BoreCylinder || r.Bounds.Lower.Cmp(big.NewRat(2, 1)) != 0 || r.Normals[0].DirectionSign != 1 {
		t.Fatal("inward cylinder normal", r, e)
	}
	n := r.Normals[0]
	nx := new(big.Rat).Add(n.Constant[0], new(big.Rat).Mul(n.Linear[0], big.NewRat(2, 1)))
	if nx.Sign() != 1 {
		t.Fatal(nx)
	}
	r, e = s.FirstBoundaryHit(ctx, Vec3{X: -6, Y: 1}, Vec3{X: 1}, 2, o)
	if e != nil || !r.HasHit || r.Boundary != BoreSphere || r.Normals[0].DirectionSign != -1 {
		t.Fatal("irrational sphere entry", r, e)
	}
	lo, hi := r.PointBounds[0].Lower, r.PointBounds[0].Upper
	if new(big.Rat).Mul(lo, lo).Cmp(big.NewRat(24, 1)) < 0 || new(big.Rat).Mul(hi, hi).Cmp(big.NewRat(24, 1)) > 0 {
		t.Fatal("independent model enclosure", r)
	}
	triple, e := NewOffAxisSphereFiniteCylinderDifference(5, 3, Vec2{X: 4}, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	r, e = triple.FirstBoundaryHit(ctx, Vec3{X: 4, Y: 3}, Vec3{X: -1}, 1, o)
	if e != nil || !r.HasHit || r.Bounds.Lower.Sign() != 0 || r.Boundary != BoreSphere|BoreCylinder|BoreLowerCap || len(r.Normals) != 3 || r.Tangent {
		t.Fatal("three active supports", r, e)
	}
	for i, mask := range []BoreBoundaryMask{BoreSphere, BoreCylinder, BoreLowerCap} {
		if r.Normals[i].Source != mask {
			t.Fatal("unstable ownership", r)
		}
	}
	if r.Normals[2].Constant[2].Sign() != 1 {
		t.Fatal("lower cutter cap outward", r)
	}
}
func TestSphereCylinderFirstUnionContact(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	s, e := NewOffAxisSphereFiniteCylinderUnion(5, 2, Vec2{X: 7}, -2, 2)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.FirstBoundaryHit(ctx, Vec3{}, Vec3{X: 1}, 10, o)
	if e != nil || !r.HasHit || r.Bounds.Lower.Cmp(big.NewRat(5, 1)) != 0 || r.Boundary != BoreSphere|BoreCylinder || len(r.Normals) != 2 || r.Isolated || r.Tangent || r.Normals[0].DirectionSign != 1 || r.Normals[1].DirectionSign != -1 {
		t.Fatal("opposed tangent-solid normals", r, e)
	}
	r, e = s.FirstBoundaryHit(ctx, Vec3{X: 5, Y: -1}, Vec3{Y: 1}, 2, o)
	if e != nil || !r.HasHit || !r.Isolated || !r.Tangent || len(r.Normals) != 2 {
		t.Fatal("line tangent to both", r, e)
	}
	s, e = NewOffAxisSphereFiniteCylinderUnion(5, 3, Vec2{X: 1}, -6, 6)
	if e != nil {
		t.Fatal(e)
	}
	r, e = s.FirstBoundaryHit(ctx, Vec3{X: 1}, Vec3{Z: 2}, 4, o)
	if e != nil || !r.HasHit || r.Boundary != BoreUpperCap || r.Bounds.Lower.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("hidden sphere not first hit", r, e)
	}
	r, e = s.FirstBoundaryHit(ctx, Vec3{X: 1}, Vec3{Z: 2}, 2, o)
	if e != nil || r.HasHit || r.Parameter != nil {
		t.Fatal("finite interior truncation", r, e)
	}
}
func TestSphereCylinderFirstHitLimits(t *testing.T) {
	s, _ := NewOffAxisSphereFiniteCylinderDifference(5, 3, Vec2{X: 1}, -2, 2)
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := s.FirstBoundaryHit(ctx, Vec3{}, Vec3{Z: 1}, 10, o)
	if e != context.Canceled || r.HasHit || r.Parameter != nil || len(r.Normals) != 0 {
		t.Fatal(r, e)
	}
	o.Surface.MaxOperations = 1
	r, e = s.FirstBoundaryHit(context.Background(), Vec3{}, Vec3{Z: 1}, 10, o)
	if e == nil || r.HasHit || r.Parameter != nil {
		t.Fatal("atomic limit", r, e)
	}
	if r, e := (OffAxisSphereFiniteCylinderDifference{}).FirstBoundaryHit(context.Background(), Vec3{}, Vec3{Z: 1}, 10, o); e == nil || r.HasHit {
		t.Fatal("invalid recipe", r, e)
	}
}
