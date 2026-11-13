package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestTransverseCylinderFirstBoundary(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	a := RawCylinder{Axis: Vec3{Z: 1}, Max: 8, Radius: 5}
	b := RawCylinder{Origin: Vec3{X: 4}, Axis: Vec3{Z: -1}, Min: -12, Max: -8, Radius: 3}
	u, _ := NewTransverseCylinderSetQuery(a, b, false)
	// Starting inside skips the shared cap at z8 and exits at original B Min z12.
	r, e := u.FirstBoundaryHit(ctx, Vec3{X: 3, Z: 4}, Vec3{Z: 2}, 5, o)
	if e != nil || !r.HasHit || r.Bounds.Lower.Cmp(big.NewRat(4, 1)) != 0 || r.Boundary != RawCylinderSecondMin || r.Isolated || r.Tangent || len(r.Normals) != 1 || r.Normals[0].Constant[2].Sign() != 1 || r.Normals[0].DirectionSign != 1 {
		t.Fatal(r, e)
	}
	if r.PointBounds[2].Lower.Cmp(big.NewRat(12, 1)) != 0 {
		t.Fatal("point enclosure", r)
	}
	// No fake hit at the finite range cap inside retained material.
	r, e = u.FirstBoundaryHit(ctx, Vec3{X: 3, Z: 4}, Vec3{Z: 2}, 3, o)
	if e != nil || r.HasHit || r.Parameter != nil {
		t.Fatal("canceled source / clipped interior", r, e)
	}
	// Simultaneous side and lower cap; each normal is retained in source bit order.
	r, e = u.FirstBoundaryHit(ctx, Vec3{X: 6, Z: -1}, Vec3{X: -1, Z: 1}, 2, o)
	if e != nil || !r.HasHit || r.Boundary != RawCylinderFirstSide|RawCylinderFirstMin || len(r.Normals) != 2 || r.Normals[0].Source != RawCylinderFirstSide || r.Normals[1].Source != RawCylinderFirstMin || r.Tangent || r.Isolated {
		t.Fatal("corner ownership", r, e)
	}
	for _, n := range r.Normals {
		if n.DirectionSign != -1 {
			t.Fatal("entering normal", n)
		}
	}
	// A clipped ray starting on a wall reports that exact boundary at t0.
	r, e = u.FirstBoundaryHit(ctx, Vec3{X: -5, Z: 4}, Vec3{X: 1}, 2, o)
	if e != nil || !r.HasHit || r.Bounds.Lower.Sign() != 0 || r.Normals[0].DirectionSign != -1 {
		t.Fatal("boundary start", r, e)
	}
	// Line tangency is an isolated material point and tangent to its sole support.
	r, e = u.FirstBoundaryHit(ctx, Vec3{X: -2, Y: 5, Z: 4}, Vec3{X: 1}, 4, o)
	if e != nil || !r.HasHit || !r.Isolated || !r.Tangent || r.Bounds.Lower.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("tangent", r, e)
	}
}
func TestTransverseCylinderFirstDifferenceNormal(t *testing.T) {
	a := RawCylinder{Axis: Vec3{Z: 1}, Max: 8, Radius: 5}
	b := RawCylinder{Origin: Vec3{X: 4}, Axis: Vec3{Z: -1}, Min: -8, Max: 0, Radius: 3}
	s, _ := NewTransverseCylinderSetQuery(a, b, true)
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	ctx := context.Background()
	r, e := s.FirstBoundaryHit(ctx, Vec3{Z: 4}, Vec3{X: 2}, 4, o)
	if e != nil || !r.HasHit || r.Boundary != RawCylinderSecondSide || r.Bounds.Lower.Cmp(big.NewRat(1, 2)) != 0 || r.Normals[0].DirectionSign != 1 {
		t.Fatal("outward cutter normal", r, e)
	}
	n := r.Normals[0]
	nx := new(big.Rat).Add(n.Constant[0], new(big.Rat).Mul(n.Linear[0], big.NewRat(1, 2)))
	if nx.Sign() <= 0 {
		t.Fatal("cutter normal not flipped", nx)
	}
	reverse, e := s.FirstBoundaryHit(ctx, Vec3{X: 3, Z: 4}, Vec3{X: -2}, 2, o)
	if e != nil || !reverse.HasHit || reverse.Bounds.Lower.Cmp(big.NewRat(1, 1)) != 0 || reverse.Boundary != RawCylinderSecondSide || reverse.Normals[0].DirectionSign != -1 {
		t.Fatal("reversed raw line", reverse, e)
	}
	// Mutation of returned coefficients cannot modify the immutable query.
	n.Constant[0].SetInt64(999)
	r, e = s.FirstBoundaryHit(ctx, Vec3{Z: 4}, Vec3{X: 2}, 4, o)
	if e != nil || r.Normals[0].Constant[0].Cmp(big.NewRat(999, 1)) == 0 {
		t.Fatal("aliased query", r, e)
	}
	// Irrational stock side entry: exact parameter root and model enclosure.
	r, e = s.FirstBoundaryHit(ctx, Vec3{X: -6, Y: 1, Z: 4}, Vec3{X: 1}, 2, o)
	if e != nil || !r.HasHit || r.Boundary != RawCylinderFirstSide || r.Normals[0].DirectionSign != -1 {
		t.Fatal("irrational entry", r, e)
	}
	lo, hi := r.PointBounds[0].Lower, r.PointBounds[0].Upper
	if new(big.Rat).Mul(lo, lo).Cmp(big.NewRat(24, 1)) < 0 || new(big.Rat).Mul(hi, hi).Cmp(big.NewRat(24, 1)) > 0 {
		t.Fatal("irrational model enclosure", r)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	r, e = s.FirstBoundaryHit(canceled, Vec3{}, Vec3{X: 1}, 10, o)
	if e != context.Canceled || r.HasHit || r.Parameter != nil || len(r.Normals) != 0 {
		t.Fatal("atomic cancel", r, e)
	}
	o.Surface.MaxOperations = 1
	r, e = s.FirstBoundaryHit(ctx, Vec3{}, Vec3{X: 1}, 10, o)
	if e == nil || r.HasHit || r.Parameter != nil {
		t.Fatal("atomic limit", r, e)
	}
}
