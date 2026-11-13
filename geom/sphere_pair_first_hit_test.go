package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSpherePairFirstBoundaryHit(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, SphereSetIntersection)
	r, e := s.FirstBoundaryHit(ctx, Vec3{Y: 3}, Vec3{X: 2}, 4, o)
	if e != nil || !r.HasHit || !r.Isolated || r.Tangent || r.Bounds.Lower.Cmp(big.NewRat(2, 1)) != 0 || r.Boundary != SpherePairFirst|SpherePairSecond || len(r.Normals) != 2 || r.Normals[0].Source != SpherePairFirst || r.Normals[1].Source != SpherePairSecond {
		t.Fatal("intersection circle", r, e)
	}
	if r.Normals[0].DirectionSign != 1 || r.Normals[1].DirectionSign != 0 {
		t.Fatal("independent active normal signs", r)
	}
	r, e = s.FirstBoundaryHit(ctx, Vec3{Y: 1}, Vec3{X: 1}, 6, o)
	if e != nil || !r.HasHit || r.Boundary != SpherePairSecond || r.Normals[0].DirectionSign != -1 {
		t.Fatal("irrational entry", r, e)
	}
	lo := new(big.Rat).Sub(big.NewRat(4, 1), r.PointBounds[0].Upper)
	hi := new(big.Rat).Sub(big.NewRat(4, 1), r.PointBounds[0].Lower)
	if new(big.Rat).Mul(lo, lo).Cmp(big.NewRat(8, 1)) > 0 || new(big.Rat).Mul(hi, hi).Cmp(big.NewRat(8, 1)) < 0 {
		t.Fatal("4-sqrt8 enclosure", r)
	}
	// Start inside: the first exposed event is A's exit, not a clipped endpoint.
	r, e = s.FirstBoundaryHit(ctx, Vec3{X: 2}, Vec3{X: 2}, 1, o)
	if e != nil || r.HasHit || r.Parameter != nil {
		t.Fatal("finite interior miss", r, e)
	}
	r, e = s.FirstBoundaryHit(ctx, Vec3{X: 2}, Vec3{X: 2}, 2, o)
	if e != nil || !r.HasHit || r.Boundary != SpherePairFirst || r.Bounds.Lower.Cmp(big.NewRat(3, 2)) != 0 || r.Normals[0].DirectionSign != 1 {
		t.Fatal("exit", r, e)
	}
	r, e = s.FirstBoundaryHit(ctx, Vec3{X: 6}, Vec3{X: -2}, 3, o)
	if e != nil || !r.HasHit || r.Boundary != SpherePairFirst || r.Bounds.Lower.Cmp(big.NewRat(1, 2)) != 0 || r.Normals[0].DirectionSign != -1 {
		t.Fatal("reversed raw ray", r, e)
	}
}
func TestSpherePairFirstSingularNormals(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 8}, 3, SphereSetUnion)
	r, e := s.FirstBoundaryHit(ctx, Vec3{X: 5, Y: -1}, Vec3{Y: 1}, 2, o)
	if e != nil || !r.HasHit || !r.Isolated || !r.Tangent || len(r.Normals) != 2 || r.Boundary != SpherePairFirst|SpherePairSecond {
		t.Fatal("external tangent union", r, e)
	}
	if r.Normals[0].Constant[0].Sign() != 1 || r.Normals[1].Constant[0].Sign() != -1 {
		t.Fatal("opposed exact normals", r)
	}
	s, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 2}, 3, SphereSetDifference)
	r, e = s.FirstBoundaryHit(ctx, Vec3{}, Vec3{X: 1}, 6, o)
	if e != nil || !r.HasHit || !r.Isolated || r.Tangent || r.Bounds.Lower.Cmp(big.NewRat(5, 1)) != 0 || len(r.Normals) != 2 || r.Normals[0].DirectionSign != 1 || r.Normals[1].DirectionSign != -1 {
		t.Fatal("regularized singular cavity contact", r, e)
	}
	n := r.Normals[1]
	value := new(big.Rat).Add(n.Constant[0], new(big.Rat).Mul(n.Linear[0], big.NewRat(5, 1)))
	if value.Sign() != -1 {
		t.Fatal("cavity normal reversal", n)
	}
	n.Constant[0].SetInt64(999)
	r, e = s.FirstBoundaryHit(ctx, Vec3{}, Vec3{X: 1}, 6, o)
	if e != nil || r.Normals[1].Constant[0].Cmp(big.NewRat(999, 1)) == 0 {
		t.Fatal("mutable return aliases source", r, e)
	}
	// Coincident union preserves both identities, while coincident difference
	// has no boundary remnants or misleading candidate normals.
	s, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{}, 5, SphereSetUnion)
	r, e = s.FirstBoundaryHit(ctx, Vec3{}, Vec3{X: 1}, 6, o)
	if e != nil || len(r.Normals) != 2 || r.Boundary != SpherePairFirst|SpherePairSecond {
		t.Fatal("coincident identity", r, e)
	}
	s, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{}, 5, SphereSetDifference)
	r, e = s.FirstBoundaryHit(ctx, Vec3{}, Vec3{X: 1}, 6, o)
	if e != nil || r.HasHit || r.Parameter != nil {
		t.Fatal("empty difference", r, e)
	}
}
func TestSpherePairFirstHitAtomicLimits(t *testing.T) {
	s, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, SphereSetDifference)
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := s.FirstBoundaryHit(ctx, Vec3{}, Vec3{X: 1}, 10, o)
	if e != context.Canceled || r.HasHit || len(r.Normals) != 0 {
		t.Fatal(r, e)
	}
	o.Surface.MaxOperations = 1
	r, e = s.FirstBoundaryHit(context.Background(), Vec3{}, Vec3{X: 1}, 10, o)
	if e == nil || r.HasHit || r.Parameter != nil {
		t.Fatal("atomic budget", r, e)
	}
	if r, e := (SpherePairMaterial{}).FirstBoundaryHit(context.Background(), Vec3{}, Vec3{X: 1}, 10, o); e == nil || r.HasHit {
		t.Fatal("invalid query", r, e)
	}
}
