package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestAffineInverseRawHitAndLosslessExport(t *testing.T) {
	ctx := context.Background()
	patch := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + 2*v} })
	chart, e := NewAffineSurfaceChart(ctx, patch, AffineSurfaceChartOptions{})
	if e != nil {
		t.Fatal(e)
	}
	spline, _ := NewBezier([]Vec3{{Z: -1}, {X: 1, Y: 1, Z: 5}}, nil)
	curve := isoIntersectionFixture(t, spline)
	roots, e := curve.IntersectRawPlane(ctx, Vec3{}, Vec3{1, 0, 1}, Vec3{0, 1, 2}, SplinePlaneOptions{})
	if e != nil || len(roots.Hits) != 1 {
		t.Fatal(roots, e)
	}
	inv, e := chart.InvertExactContext(ctx, roots.Hits[0].Point, AffineChartInverseOptions{})
	if e != nil || !inv.Inside || !inv.OnPlane || inv.U.Cmp(big.NewRat(1, 3)) != 0 || inv.V.Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(inv, e)
	}
	if _, e = inv.RepresentedUV(); !errors.Is(e, ErrUnrepresentable) {
		t.Fatal(e)
	}
	if _, e = RepresentedExactPoint(roots.Hits[0].Point); !errors.Is(e, ErrUnrepresentable) {
		t.Fatal(e)
	}
	exact, _ := chart.EvalExact(big.NewRat(1, 2), big.NewRat(1, 4))
	inv, e = chart.InvertExactContext(ctx, exact, AffineChartInverseOptions{})
	if e != nil {
		t.Fatal(e)
	}
	uv, e := inv.RepresentedUV()
	if e != nil || uv != (Vec2{.5, .25}) {
		t.Fatal(uv, e)
	}
	point, e := RepresentedExactPoint(exact)
	if e != nil || point != (Vec3{.5, .25, 1}) {
		t.Fatal(point, e)
	}
	outside, _ := chart.EvalExact(big.NewRat(2, 1), big.NewRat(-1, 1))
	inv, e = chart.InvertExactContext(ctx, outside, AffineChartInverseOptions{})
	if e != nil || inv.Inside || !inv.OnPlane {
		t.Fatal(inv, e)
	}
	_, e = chart.InvertExactContext(ctx, exact, AffineChartInverseOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = chart.InvertExactContext(canceled, exact, AffineChartInverseOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e = RepresentedExactPoint([3]*big.Rat{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
