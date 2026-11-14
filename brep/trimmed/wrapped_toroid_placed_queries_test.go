package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestPlacedWrappedToroidRationalLine(t *testing.T) {
	ctx := context.Background()
	recipe, e := independentWrappedToroid(t).ExportRecipe()
	if e != nil {
		t.Fatal(e)
	}
	d := recipe.Surface.Data()
	weights := [9]float64{1, 1, 2, 2, 4, 2, 2, 1, 1}
	for i := range weights {
		for j := range weights {
			d.Weights[9*i+j] = weights[i] * weights[j]
		}
	}
	surface, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(ctx, patch, recipe.Options)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := NewWrappedToroid(ctx, w, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q := big.NewRat
	z := q(0, 1)
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{q(3, 5), z, q(4, 5), q(1, 7)}, {z, q(1, 1), z, q(1, 11)}, {q(-4, 5), z, q(3, 5), q(2, 13)}}, 0)
	if e != nil {
		t.Fatal(e)
	}
	o := geom.ToroidLineOptions{ParameterTolerance: 1e-15}
	origin := [3]*big.Rat{q(1, 7), q(1, 11), q(2, 13)}
	dir := [3]*big.Rat{q(9, 35), z, q(-12, 35)} // local X scale 3/7
	r, e := s.IntersectPlacedWrappedToroidLineRational(ctx, pose, origin, dir, q(-7, 1), q(7, 1), o)
	if e != nil || len(r) != 4 {
		t.Fatal(r, e)
	}
	for i, want := range [4]*big.Rat{q(-35, 6), q(-7, 2), q(7, 2), q(35, 6)} {
		if r[i].Min.Cmp(want) > 0 || r[i].Max.Cmp(want) < 0 || r[i].Multiplicity != 1 {
			t.Fatal("four independent meridian contacts in original line units", i, r[i], want)
		}
	}
	// p is the independently transformed local (2,0,1/2), the exact
	// top of the torus. Its Y tangent has fourth order contact. Move
	// by a sub-ULP local radial coordinate and the contact disappears.
	tangent := [3]*big.Rat{q(61, 35), q(1, 11), q(-149, 130)}
	dir = [3]*big.Rat{z, q(2, 3), z}
	r, e = s.IntersectPlacedWrappedToroidLineRational(ctx, pose, tangent, dir, q(-1, 3), q(1, 3), o)
	if e != nil || len(r) != 1 || r[0].Multiplicity != 4 || r[0].Min.Sign() != 0 || r[0].Max.Sign() != 0 {
		t.Fatal("fourth order exact rational tangent", r, e)
	}
	delta := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	tangent[0].Add(tangent[0], new(big.Rat).Mul(delta, q(3, 5)))
	tangent[2].Sub(tangent[2], new(big.Rat).Mul(delta, q(4, 5)))
	r, e = s.IntersectPlacedWrappedToroidLineRational(ctx, pose, tangent, dir, q(-1, 3), q(1, 3), o)
	if e != nil || len(r) != 0 {
		t.Fatal("distinct line outside radial tangent", r, e)
	}
	lim := o
	lim.Surface.MaxCoefficientBits = 64
	if r, e = s.IntersectPlacedWrappedToroidLineRational(ctx, pose, tangent, dir, q(-1, 3), q(1, 3), lim); !errors.Is(e, geom.ErrConvergence) || r != nil {
		t.Fatal("partial budget result", r, e)
	}
	// A different radial probe lies exactly epsilon away from the outer
	// equator. Its true signed squared distance is ±2^-240, with one
	// original world toroid face and original local meridian witness.
	for _, sign := range []int64{-1, 0, 1} {
		localX := new(big.Rat).Add(q(5, 2), new(big.Rat).Mul(q(sign, 1), delta))
		world := [3]*big.Rat{new(big.Rat).Add(origin[0], new(big.Rat).Mul(q(3, 5), localX)), origin[1], new(big.Rat).Add(origin[2], new(big.Rat).Mul(q(-4, 5), localX))}
		projected, e := s.ProjectPlacedCircularToroidRational(ctx, pose, world, geom.FrustumDistanceOptions{SquaredTolerance: 1e-84, MaxBisections: 512})
		if e != nil {
			t.Fatal(sign, e)
		}
		want := new(big.Rat).Mul(q(sign, 1), new(big.Rat).Mul(delta, delta))
		loc := [3]geom.ToroidMeridianLocation{geom.ToroidMeridianInside, geom.ToroidMeridianBoundary, geom.ToroidMeridianOutside}[sign+1]
		p := projected.Local
		if p.Location != loc || p.SignedSquaredMin.Cmp(want) != 0 || p.SignedSquaredMax.Cmp(want) != 0 || projected.Face != 0 ||
			p.Distance.AzimuthX.Cmp(localX) != 0 || p.Distance.AzimuthY.Sign() != 0 || p.Distance.HeightOffset.Sign() != 0 {
			t.Fatal("sub-ULP signed toroid distance and original witness", sign, projected, want)
		}
		for i, j := range origin {
			if projected.Center[i].Cmp(j) != 0 {
				t.Fatal("world witness frame lost exact translation", projected)
			}
		}
	}

	offPlane, e := s.SectionPlacedCircularToroidRational(ctx, pose, q(1, 3), geom.SurfacePatchOptions{})
	if e != nil || !offPlane.Local.Present || offPlane.Local.SingleCircle || offPlane.Face != 0 ||
		offPlane.Local.MajorRadius.Cmp(q(2, 1)) != 0 || offPlane.Local.RadialOffsetSquared.Cmp(q(5, 36)) != 0 ||
		offPlane.Local.AreaPiSqrtCoefficient.Cmp(q(8, 1)) != 0 {
		t.Fatal("exact nonbinary annular section", offPlane, e)
	}
	for i, p := range [3]*big.Rat{q(43, 105), q(1, 11), q(23, 65)} {
		if offPlane.Center[i].Cmp(p) != 0 {
			t.Fatal("original world annulus center", i, offPlane.Center)
		}
	}
	pole, e := s.SectionPlacedCircularToroidRational(ctx, pose, q(1, 2), geom.SurfacePatchOptions{})
	if e != nil || !pole.Local.Present || !pole.Local.SingleCircle || pole.Local.RadialOffsetSquared.Sign() != 0 {
		t.Fatal("outer section is a single exact circle", pole, e)
	}
	empty, e := s.SectionPlacedCircularToroidRational(ctx, pose, new(big.Rat).Add(q(1, 2), delta), geom.SurfacePatchOptions{})
	if e != nil || empty.Local.Present || empty.Face != -1 {
		t.Fatal("sub-ULP axial miss", empty, e)
	}

}
