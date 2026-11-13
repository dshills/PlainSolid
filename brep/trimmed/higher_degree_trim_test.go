package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestRationalCubicTrimProof(t *testing.T) {
	ctx := context.Background()
	a, b, c, d := geom.Vec2{}, geom.Vec2{X: 4}, geom.Vec2{X: 4, Y: 3}, geom.Vec2{Y: 3}
	f := Face{Outer: []Use{{UVStart: a, UVEnd: b}, {UVStart: b, UVEnd: c},
		{UVStart: c, UVEnd: d}, {UVStart: d, UVEnd: a}}}
	makeCurve := func(x1, x2, y float64) SplineUV {
		t.Helper()
		p, e := geom.NewBezier([]geom.Vec3{{}, {X: x1, Y: y}, {X: x2, Y: y}, {X: 4}},
			[]float64{1, .75, .75, 1})
		if e != nil {
			t.Fatal(e)
		}
		return SplineUV{Curve: p, Range: UVInterval{Max: 1}}
	}
	f.Outer[0].Curve = makeCurve(1, 3, -2)
	p, e := ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
	if e != nil || p.Status != trimvalidate.Certified {
		t.Fatal(p, e)
	}
	// Rational cubic at t=1/2: x=2, y=(-2)*(9/13).
	location, p, e := LocateRationalInFaceTrim(ctx, f, [2]*big.Rat{big.NewRat(2, 1), big.NewRat(-18, 13)}, trimvalidate.Options{})
	if e != nil || p.Status != trimvalidate.Certified || location != trimvalidate.Boundary {
		t.Fatal(location, p, e)
	}
	f.Outer[0].Curve = makeCurve(1, 3, 6)
	p, e = ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
	if e != nil || p.Status != trimvalidate.Invalid {
		t.Fatal("curve crosses the opposite line", p, e)
	}
	f.Outer[0].Curve = makeCurve(3, 1, -2)
	p, e = ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
	if e != nil || p.Status != trimvalidate.Certified || p.Subdivisions == 0 {
		t.Fatal("monotone only after exact refinement", p, e)
	}
	f.Outer[0].Curve = makeCurve(7, -3, -2)
	p, e = ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
	if e != nil || p.Status != trimvalidate.Unresolved {
		t.Fatal("no monotone control projection", p, e)
	}
}
