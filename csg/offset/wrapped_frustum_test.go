package offset

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
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

func TestOpenWrappedCircularFrustumWall(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	base, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: -1, Y: 2, Z: 3}, 2, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range [][2]float64{{0, 3}, {3, 0}} {
		outer, _, e := geom.DraftWrappedFrustum(ctx, base, change[0], change[1], o)
		if e != nil {
			t.Fatal(e)
		}
		for _, openUpper := range []bool{false, true} {
			inner, proof, e := geom.InsetWrappedCircularFrustumAtCap(ctx, outer, .5, openUpper, o)
			if e != nil {
				t.Fatal(e)
			}
			z, radii := [2]float64{3, 6.5}, [2]float64{11.0 / 8, 4}
			if openUpper {
				z, radii = [2]float64{3.5, 7}, [2]float64{1.75, 35.0 / 8}
			}
			if change[0] > change[1] {
				radii = [2]float64{35.0 / 8, 1.75}
				if openUpper {
					radii = [2]float64{4, 11.0 / 8}
				}
			}
			for i, q := range [2]*big.Rat{proof.LowerZ, proof.UpperZ} {
				if q.Cmp(new(big.Rat).SetFloat64(z[i])) != 0 ||
					[2]*big.Rat{proof.LowerRadius, proof.UpperRadius}[i].Cmp(new(big.Rat).SetFloat64(radii[i])) != 0 {
					t.Fatal("offset support and authored cap plane", openUpper, proof, radii)
				}
			}
			body, h, e := WrappedCircularFrustumOpenWall(ctx, outer, .5, 1e-8, openUpper, o)
			if e != nil || len(body.Faces()) != 5 || len(body.Topology().Shells) != 1 ||
				body.Topology().InnerWires != 1 || body.Topology().EulerCharacteristic != 2 {
				t.Fatal("open cup topology", body.Topology(), e)
			}
			cap, floor := 1, 2
			if openUpper {
				cap, floor = 2, 1
			}
			if h.InnerFaces[cap] != h.OuterFaces[cap] || h.InnerFaces[floor] == h.OuterFaces[floor] {
				t.Fatal("opening and floor ancestry", h)
			}
			f := body.Faces()
			if f[0].Surface != outer || !f[3].Reversed || len(f[cap].Holes) != 1 {
				t.Fatal("original surface and annular cap use")
			}
			q, _, e := trimmed.NewWrappedCircularFrustumBore(ctx, outer, inner, 1e-8, o)
			if e != nil || q.Faces()[3].Surface != inner {
				t.Fatal("direct bore retains original tool support", e)
			}
			open, bottom := 0, 1
			if openUpper {
				open, bottom = 1, 0
			}
			for _, p := range []struct {
				z, radial float64
				want      parametric.Location
			}{{z[open], 0, parametric.Outside}, {z[open], radii[open], parametric.Boundary},
				{z[open], radii[open] + .25, parametric.Boundary}, {z[bottom], 0, parametric.Boundary},
				{(z[0] + z[1]) / 2, 0, parametric.Outside}, {(z[0] + z[1]) / 2, (radii[0] + radii[1] + .5) / 2, parametric.Inside}} {
				loc, e := trimmed.LocateWrappedCircularFrustumBore(ctx, outer, inner, geom.Vec3{X: -1 + p.radial, Y: 2, Z: p.z}, o)
				if e != nil || loc != p.want {
					t.Fatal("independent material classification", p, loc, e)
				}
			}
			coeff := func(a, b float64) float64 { return (a*a + a*b + b*b) / 3 }
			want := math.Pi * (52 - (z[1]-z[0])*coeff(radii[0], radii[1]))
			mass, e := body.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(mass.Volume-want) > 1e-8 {
				t.Fatal("independent volume", mass, want, e)
			}
		}
	}
	if body, _, e := WrappedCircularFrustumOpenWall(ctx, base, 2, 1e-8, true, o); !errors.Is(e, geom.ErrDomain) || len(body.Faces()) != 0 {
		t.Fatal("vanishing inner radius returned a partial wall", e)
	}
	if body, _, e := trimmed.NewWrappedCircularFrustumBore(ctx, base, base, 1e-8, o); !errors.Is(e, geom.ErrConvergence) || len(body.Faces()) != 0 {
		t.Fatal("radial contact returned a partial native wall", e)
	}
}

func TestEccentricOpenWrappedCircularFrustumBore(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: -1, Y: 2, Z: 3}, 2, 4, o)
	if e != nil {
		t.Fatal(e)
	}
	outer, _, e = geom.DraftWrappedFrustum(ctx, outer, 0, 3, o)
	if e != nil {
		t.Fatal(e)
	}
	local, _, e := geom.InsetWrappedCircularFrustumAtCap(ctx, outer, .5, true, o)
	if e != nil {
		t.Fatal(e)
	}
	translate := func(x, y float64) *geom.WrappedSurfacePatch {
		t.Helper()
		d := geom.Identity()
		d[0][3], d[1][3] = x, y
		tool, _, e := local.Transformed(ctx, d, geom.WrappedSurfaceOptions{WrapU: true})
		if e != nil {
			t.Fatal(e)
		}
		return tool
	}
	// The offset distance is sqrt(2)/4. The exact radial gap at
	// either tool endpoint is 5/8, so no square root is needed.
	tool := translate(.25, .25)
	body, history, e := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, outer, tool, 1e-8, o)
	if e != nil || len(body.Faces()) != 5 || body.Topology().EulerCharacteristic != 2 || len(body.Topology().Shells) != 1 ||
		body.Topology().InnerWires != 1 || history.OuterFaces[2] != history.InnerFaces[2] {
		t.Fatal("eccentric opening shape and cap ancestry", body.Topology(), history, e)
	}
	if f := body.Faces(); f[0].Surface != outer || f[3].Surface != tool || !f[3].Reversed {
		t.Fatal("original eccentric tool chart lost")
	}
	for _, p := range []struct {
		point geom.Vec3
		want  parametric.Location
	}{
		{geom.Vec3{X: -.75, Y: 2.25, Z: 7}, parametric.Outside},
		{geom.Vec3{X: 29.0 / 8, Y: 2.25, Z: 7}, parametric.Boundary},
		{geom.Vec3{X: -.75, Y: 2.25, Z: 3.5}, parametric.Boundary},
		{geom.Vec3{X: -.75, Y: 2.25, Z: 3.25}, parametric.Inside},
	} {
		where, e := trimmed.LocateWrappedEccentricCircularFrustumBore(ctx, outer, tool, p.point, o)
		if e != nil || where != p.want {
			t.Fatal("original-axis location", p, where, e)
		}
	}
	mass, e := body.MassProperties(ctx, parametric.IntegrationOptions{})
	toolCoeff := 3.5 * (1.75*1.75 + 1.75*4.375 + 4.375*4.375) / 3
	wantVolume := (52 - toolCoeff) * math.Pi
	wantX := (-52 + .75*toolCoeff) / (52 - toolCoeff)
	wantY := (104 - 2.25*toolCoeff) / (52 - toolCoeff)
	if e != nil || math.Abs(mass.Volume-wantVolume) > 1e-8 || math.Abs(mass.Centroid.X-wantX) > 1e-8 || math.Abs(mass.Centroid.Y-wantY) > 1e-8 {
		t.Fatal("independent eccentric volume and first moment", mass, wantVolume, wantX, wantY, e)
	}
	if bad, _, e := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, outer, translate(.625, 0), 1e-8, o); !errors.Is(e, geom.ErrConvergence) || len(bad.Faces()) != 0 {
		t.Fatal("radial tangency must reject without a body", e)
	}
}
