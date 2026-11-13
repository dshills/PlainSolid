package parametric

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestSphereFiniteOffsetCylinderCommon(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, c := range []struct {
		lo, hi, d float64
		want      SphereCylinderCommonRegime
		spherical [2]bool
	}{
		{-1, 2, 2, SphereCylinderCommonFullEnds, [2]bool{}},
		{-6, 2, 2, SphereCylinderCommonFullEnds, [2]bool{true, false}},
		{-2, 4.75, 2, SphereCylinderCommonOneLens, [2]bool{false, true}},
		{-4.75, 2, 2, SphereCylinderCommonOneLens, [2]bool{true, false}},
		{-6, 4.75, 2, SphereCylinderCommonOneLens, [2]bool{true, true}},
		{-4.75, 6, 2, SphereCylinderCommonOneLens, [2]bool{true, true}},
		{-4.25, 4.75, 2, SphereCylinderCommonOppositeLenses, [2]bool{true, true}},
		{4.125, 4.75, 2, SphereCylinderCommonSameSideLenses, [2]bool{false, true}},
		{-4.75, -4.125, 2, SphereCylinderCommonSameSideLenses, [2]bool{true, false}},
		{5, 6, 2, SphereCylinderCommonEmpty, [2]bool{}},
		{-6, 6, 6, SphereCylinderCommonEmpty, [2]bool{}},
		{-6, 6, 7, SphereCylinderCommonEmpty, [2]bool{}},
	} {
		r, e := SphereFiniteOffsetCylinderCommon(ctx, f, 5, 1, c.d, c.lo, c.hi, 1e-9)
		if e != nil || r.Regime != c.want {
			t.Fatal(c, r.Regime, e)
		}
		if c.want == SphereCylinderCommonEmpty {
			if len(r.Solid.Faces()) != 0 {
				t.Fatal(c, "nonempty result")
			}
			continue
		}
		for i, yes := range c.spherical {
			if (len(r.SphericalEndFaces[i]) > 0) != yes || !r.CapMaterial[i].Complete {
				t.Fatal(c, i, r.SphericalEndFaces)
			}
		}
		if c.want == SphereCylinderCommonOneLens {
			p, e := r.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
			if e != nil || p.Volume <= 0 || math.Signbit(p.Centroid.Z) != (c.lo+c.hi < 0) {
				t.Fatal(c, p, e)
			}
		}
	}
	r, e := SphereFiniteOffsetCylinderCommon(ctx, f, 5, 1, .5, -4.99, 4.85, 1e-9)
	if !errors.Is(e, geom.ErrConvergence) || len(r.Solid.Faces()) != 0 {
		t.Fatal("unimplemented sphere-only disk and lens was accepted", e)
	}
}

func TestSphereFiniteOffsetCylinderCommonCoveredSlab(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, c := range []struct {
		r, d, lo, hi float64
		caps         [2]bool
	}{
		{7, 1, -2, 3, [2]bool{true, true}},  // two irrational planar rims
		{6, 1, -2, 7, [2]bool{true, false}}, // equatorial tangency and a pole
		{4, 1, 4, 6, [2]bool{true, false}},  // tangency at the nearest slice
		{7, 1, -7, 7, [2]bool{}},            // full sphere
	} {
		out, e := SphereFiniteOffsetCylinderCommon(ctx, f, 5, c.r, c.d, c.lo, c.hi, 1e-8)
		if e != nil || out.Regime != SphereCylinderCommonSphericalSlab || len(out.WallFaces) != 0 || len(out.SphericalZoneFaces) != 4 {
			t.Fatal(c, out.Regime, e)
		}
		for i, yes := range c.caps {
			if (len(out.PlanarEndFaces[i]) > 0) != yes {
				t.Fatal(c, out.PlanarEndFaces)
			}
		}
		p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		primitive := func(z float64) float64 { return 25*z - z*z*z/3 }
		want := math.Pi * (primitive(min(5, c.hi)) - primitive(max(-5, c.lo)))
		if e != nil || math.Abs(p.Volume-want) > 2e-7 {
			t.Fatal(c, p.Volume, want, e)
		}
	}
}
