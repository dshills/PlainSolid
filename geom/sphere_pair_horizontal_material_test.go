package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSpherePairHorizontalMaterialConsistency(t *testing.T) {
	ctx := context.Background()
	model, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, SphereSetIntersection)
	section, e := model.HorizontalMaterial(1)
	if e != nil || section.Height() != 1 {
		t.Fatal(section, e)
	}
	for _, p := range []Vec2{{X: 2}, {X: 4, Y: 2}, {X: 5}, {Y: 1}, {X: 4, Y: 3}} {
		a, am, ae := section.Classify(p)
		b, bm, be := model.Classify(Vec3{X: p.X, Y: p.Y, Z: 1})
		if ae != nil || be != nil || a != b || am != bm {
			t.Fatal("global point mismatch", p, a, am, b, bm, ae, be)
		}
	}
	r, e := section.ClipLine(ctx, Vec2{}, Vec2{X: 1}, -10, 10, ToroidLineOptions{ParameterTolerance: 1e-10})
	if e != nil || len(r.Intervals) != 1 || len(r.Points) != 0 {
		t.Fatal(r, e)
	}
	iv := r.Intervals[0]
	sq := func(v *big.Rat) *big.Rat { return new(big.Rat).Mul(v, v) }
	lo := new(big.Rat).Sub(big.NewRat(4, 1), iv.StartBounds.Upper)
	hi := new(big.Rat).Sub(big.NewRat(4, 1), iv.StartBounds.Lower)
	if sq(lo).Cmp(big.NewRat(8, 1)) > 0 || sq(hi).Cmp(big.NewRat(8, 1)) < 0 || sq(iv.EndBounds.Lower).Cmp(big.NewRat(24, 1)) > 0 || sq(iv.EndBounds.Upper).Cmp(big.NewRat(24, 1)) < 0 || iv.StartBoundary != SpherePairSecond || iv.EndBoundary != SpherePairFirst {
		t.Fatal("independent projected disk lens", iv)
	}
	support, e := section.Supports(ctx, SurfacePatchEvaluationOptions{})
	if e != nil || support.Supports[0].RadiusSquared.Cmp(big.NewRat(24, 1)) != 0 || support.Supports[1].RadiusSquared.Cmp(big.NewRat(8, 1)) != 0 {
		t.Fatal(support, e)
	}
	support.Height.SetInt64(99)
	support.Supports[0].RadiusSquared.SetInt64(99)
	if section.Height() != 1 {
		t.Fatal("mutable height")
	}
	again, e := section.Supports(ctx, SurfacePatchEvaluationOptions{})
	if e != nil || again.Supports[0].RadiusSquared.Cmp(big.NewRat(24, 1)) != 0 {
		t.Fatal("mutable source", again, e)
	}
}
func TestSpherePairHorizontalIsolatedGlobalMaterial(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	model, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, SphereSetIntersection)
	section, _ := model.HorizontalMaterial(3)
	loc, mask, e := section.Classify(Vec2{X: 4})
	if e != nil || loc != BoreBoundary || mask != SpherePairFirst|SpherePairSecond {
		t.Fatal("point section must not be erased", loc, mask, e)
	}
	r, e := section.ClipLine(ctx, Vec2{}, Vec2{X: 1}, 0, 8, o)
	if e != nil || len(r.Intervals) != 0 || len(r.Points) != 1 || r.Points[0].Bounds.Lower.Cmp(big.NewRat(4, 1)) != 0 {
		t.Fatal("isolated section contact", r, e)
	}
	hit, e := section.FirstBoundaryHit(ctx, Vec2{}, Vec2{X: 1}, 8, o)
	if e != nil || !hit.HasHit || !hit.Isolated || len(hit.Normals) != 2 || hit.PointBounds[2].Lower.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("3D normal provenance", hit, e)
	}
	// At the pole of a retained sphere the true normal has zero planar part.
	model, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 10}, 1, SphereSetUnion)
	section, _ = model.HorizontalMaterial(5)
	hit, e = section.FirstBoundaryHit(ctx, Vec2{X: -1}, Vec2{X: 1}, 2, o)
	if e != nil || !hit.HasHit || !hit.Isolated || !hit.Tangent || len(hit.Normals) != 1 || hit.Normals[0].Constant[2].Sign() != 1 {
		t.Fatal("pole normal retained", hit, e)
	}
	model, _ = NewSpherePairMaterial(Vec3{}, 5, Vec3{}, 5, SphereSetDifference)
	section, _ = model.HorizontalMaterial(5)
	r, e = section.ClipLine(ctx, Vec2{X: -1}, Vec2{X: 1}, 0, 2, o)
	if e != nil || len(r.Intervals)+len(r.Points) != 0 {
		t.Fatal("globally removed pole", r, e)
	}
}
func TestSpherePairHorizontalInvalidAndCancellation(t *testing.T) {
	model, _ := NewSpherePairMaterial(Vec3{}, 5, Vec3{X: 4}, 3, SphereSetUnion)
	if _, e := model.HorizontalMaterial(math.NaN()); e == nil {
		t.Fatal("nonfinite height")
	}
	if _, _, e := (SpherePairHorizontalMaterial{}).Classify(Vec2{}); e == nil {
		t.Fatal("unbound view")
	}
	section, _ := model.HorizontalMaterial(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := section.ClipLine(ctx, Vec2{}, Vec2{X: 1}, 0, 10, ToroidLineOptions{ParameterTolerance: 1e-9})
	if e != context.Canceled || len(r.Intervals)+len(r.Points) != 0 {
		t.Fatal("atomic cancel", r, e)
	}
}
