package offset

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestWrappedCircularFrustumNormalOffset(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	c, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 1, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.DraftWrappedFrustum(ctx, c, 0, 3, o)
	if e != nil {
		t.Fatal(e)
	}
	s, proof, e := WrappedCircularFrustum(ctx, w, 1.25, 1e-9, o)
	if e != nil || proof.RadiusShift.Cmp(big.NewRat(1, 1)) != 0 ||
		proof.HeightShift.Cmp(big.NewRat(-3, 4)) != 0 || proof.DistanceSquared.Cmp(big.NewRat(25, 16)) != 0 {
		t.Fatal(proof, e)
	}
	a, e := w.EvaluateRational(ctx, big.NewRat(1, 3), big.NewRat(2, 7), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := proof.Support.EvaluateRational(ctx, big.NewRat(1, 3), big.NewRat(2, 7), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	d, du, dv := new(big.Rat), new(big.Rat), new(big.Rat)
	for i := range a.Point {
		x := new(big.Rat).Sub(b.Point[i], a.Point[i])
		d.Add(d, new(big.Rat).Mul(x, x))
		du.Add(du, new(big.Rat).Mul(x, a.Du[i]))
		dv.Add(dv, new(big.Rat).Mul(x, a.Dv[i]))
	}
	if d.Cmp(big.NewRat(25, 16)) != 0 || du.Sign() != 0 || dv.Sign() != 0 {
		t.Fatal("not an exact constant normal displacement", d, du, dv)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-52*math.Pi) > 1e-8 {
		t.Fatal(p, e)
	}
	if _, _, e = WrappedCircularFrustum(ctx, w, -1.25, 1e-9, o); !errors.Is(e, geom.ErrDegenerate) {
		t.Fatal("offset collapsed a radius", e)
	}
	w, _, e = geom.DraftWrappedFrustum(ctx, c, 0, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = geom.OffsetWrappedCircularFrustum(ctx, w, 1, o); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("rounded irrational normal scale", e)
	}
}
