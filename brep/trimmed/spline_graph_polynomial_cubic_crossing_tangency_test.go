package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func nativeCubicCrossingTangencyFixture(t *testing.T, axis, profile int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	// Subtract f to get -(u-1/4)^3(u-3/4)^2, -(u-3/4)^3(u-1/4)^2,
	// or -(5u-1)^3(4u-3)^2/1024 for the nonbinary crossing control.
	g := [3][]*big.Rat{
		{big.NewRat(265, 1024), big.NewRat(63, 256), big.NewRat(11, 32), big.NewRat(-7, 4), big.NewRat(9, 4), big.NewRat(-1, 1)},
		{big.NewRat(283, 1024), big.NewRat(15, 256), big.NewRat(33, 32), big.NewRat(-11, 4), big.NewRat(11, 4), big.NewRat(-1, 1)},
		{big.NewRat(265, 1024), big.NewRat(225, 1024), big.NewRat(667, 1024), big.NewRat(-3037, 1024), big.NewRat(4200, 1024), big.NewRat(-2000, 1024)},
	}
	return twoPolynomialRoofFixture(t, axis, f, g[profile])
}

func polynomialCubicCrossingTangencyEnvelopeMeasures(t *testing.T, profile int, op parametric.BooleanOp) (area, volume float64) {
	t.Helper()
	// Direct source integration uses (105/128)(v³/3-(f+g)v²/2+fgv)
	// at the original branch and perimeter ordinates on each free slab.
	between := [2][3]*big.Rat{
		{big.NewRat(4909741, 2515682604351488), big.NewRat(1, 38386270208), big.NewRat(160842989, 2515682604351488)},
		{big.NewRat(160842989, 2515682604351488), big.NewRat(1, 38386270208), big.NewRat(4909741, 2515682604351488)},
	}
	exterior := [2][2]*big.Rat{
		{big.NewRat(28550399214829, 2515682604351488), big.NewRat(590037023506439, 7547047813054464)},
		{big.NewRat(86673486561287, 7547047813054464), big.NewRat(195276624577773, 2515682604351488)},
	}
	sum := func(values []*big.Rat) *big.Rat {
		total := new(big.Rat)
		for _, v := range values {
			if v.Sign() <= 0 {
				t.Fatal("each original source contribution is positive", v)
			}
			total.Add(total, v)
		}
		return total
	}
	exact := big.NewRat(769, 256) // integral of B=3+u*v/64
	fuse := op == parametric.Union
	if fuse {
		exact.Add(exact, sum(exterior[profile][:]))
		want := [2]*big.Rat{big.NewRat(11673156157917031, 3773523906527232), big.NewRat(11671563727488871, 3773523906527232)}
		if exact.Cmp(want[profile]) != 0 {
			t.Fatal("exact original Fuse material", exact)
		}
	} else {
		exact.Sub(exact, sum(between[profile][:]))
		if exact.Cmp(big.NewRat(3778437266204723, 1257841302175744)) != 0 {
			t.Fatal("exact original Common material", exact)
		}
	}
	area = polynomialPairSourceEnvelopeArea(fuse, []float64{0, .25, .75, 1}, cubicCrossingTangencyBranches(profile))
	volume, _ = exact.Float64()
	return area, volume
}

func cubicCrossingTangencyBranches(profile int) func(float64) (f, g, df, dg float64) {
	cubic, tangent := [2]float64{.25, .75}[profile], [2]float64{.75, .25}[profile]
	return func(u float64) (f, g, df, dg float64) {
		f = ((u/8-3.0/8)*u+3.0/8)*u + .25
		df = (3*u/8-.75)*u + .375
		x, y := u-cubic, u-tangent
		g = f - x*x*x*y*y
		dg = df - x*x*y*(3*y+2*x)
		return f, g, df, dg
	}
}

func TestNativePolynomialCubicCrossingTangencyEnvelopeRejectsOtherArrangement(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativeSimpleCubicCrossingsFixture(t, 0, 0)
	pa, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pb, e := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := geom.DiscoverSplineGraphPolynomialArrangement(ctx, pa, pb, 0, geom.GraphPointContactOptions{})
	if e != nil || !proof.Proven {
		t.Fatal("other profile is independently proved by generic geometry", e)
	}
	out, e := CombineSplineGraphPolynomialCubicCrossingTangency(ctx, base, a, b, 0, parametric.Intersection, 1e-9, PolynomialCubicCrossingTangencyEnvelopeOptions{})
	if out != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("bounded cubic crossing and tangency entry must reject other multiplicities", e)
	}
}
