package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

// Exact homogeneous translation changes geometry, not the source weights or
// projective parameterization. This independent fixture deliberately puts all
// first moments and centroid coordinates on the negative branch of division.
func TestRationalGraphStockTranslatedNegativeCentroid(t *testing.T) {
	ctx := context.Background()
	rat := big.NewRat
	shift := [3]int64{-3, -5, -2}
	source := reparameterizedGraphSleeveForFlux(t)
	var parts []SewFace
	for _, f := range source.Faces() {
		p, e := exactFaceSupportPatch(f, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		recipe, e := p.ExportExactBezierRecipe()
		if e != nil {
			t.Fatal(e)
		}
		for i := range recipe.Controls {
			w, ok := new(big.Rat).SetString(recipe.Controls[i][3])
			if !ok {
				t.Fatal("weight")
			}
			for axis := range 3 {
				x, ok := new(big.Rat).SetString(recipe.Controls[i][axis])
				if !ok {
					t.Fatal("control")
				}
				x.Add(x, new(big.Rat).Mul(rat(shift[axis], 1), w))
				recipe.Controls[i][axis] = x.RatString()
			}
		}
		patch, e := geom.ImportExactBezierSurfaceRecipe(ctx, recipe, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		surface, e := geom.NewExactBezierSurface(patch)
		if e != nil {
			t.Fatal(e)
		}
		part, e := ExactSplineFaceRectangle(ctx, surface, geom.ParameterRectangle{}, f.Reversed, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		parts = append(parts, part)
	}
	stock, _, e := Sew(ctx, parts, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	const z = .5 - 11./64
	planes := [2][3]geom.Vec3{{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}}, {{Z: z}, {X: 1, Z: z}, {Y: 1, Z: z + 1./8}}}
	for i := range planes {
		for j := range planes[i] {
			planes[i][j] = planes[i][j].Add(geom.Vec3{X: -3, Y: -5, Z: -2})
		}
	}
	options := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	pair, e := stock.ClipAlgebraicGraphCapPair(ctx, planes, options, RationalGraphShellRegionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	width := rat(1, 1<<15)
	mass, e := pair.BoundVerticalCentroid(ctx, width, options, geom.ExactCurvePlaneEventOptions{MaxOperations: 20000000, MaxCoefficientBits: 32768, MaxSubdivisions: 8192})
	if e != nil {
		t.Fatal(e)
	}
	// Independently integrate h=(y-11/8)/8 over x in[0,3],
	// y in[11/8,3+x²/8], subtracting the bore for x in[1,2]
	// with y in[max(11/8,1+x²/8),2+x²/8]. Its lower limit changes
	// at sqrt(3). Densities are h,x*h,y*h,(h+h²)/2.
	// Each resulting unplaced measure is A-B*sqrt(3).
	reference := [4][2]*big.Rat{{rat(697, 960), rat(3, 640)}, {rat(7939, 6144), rat(0, 1)}, {rat(7349, 3584), rat(39, 7168)}, {rat(184411, 430080), rat(327, 143360)}}
	decimal := func(s string) *big.Rat {
		v, ok := new(big.Rat).SetString(s)
		if !ok {
			t.Fatal(s)
		}
		return v
	}
	rootLo, rootHi := decimal("1.732050807568877293527446341505"), decimal("1.732050807568877293527446341506")
	if new(big.Rat).Mul(rootLo, rootLo).Cmp(rat(3, 1)) > 0 || new(big.Rat).Mul(rootHi, rootHi).Cmp(rat(3, 1)) < 0 {
		t.Fatal("independent root brackets")
	}
	interval := func(a, b *big.Rat) geom.ExactParameterInterval {
		lo, hi := rootHi, rootLo
		if b.Sign() < 0 {
			lo, hi = hi, lo
		}
		return geom.ExactParameterInterval{Lower: new(big.Rat).Sub(a, new(big.Rat).Mul(b, lo)), Upper: new(big.Rat).Sub(a, new(big.Rat).Mul(b, hi))}
	}
	contains := func(got, want geom.ExactParameterInterval) {
		t.Helper()
		if got.Lower.Cmp(want.Lower) > 0 || got.Upper.Cmp(want.Upper) < 0 {
			t.Fatal("independent translated enclosure", got, want)
		}
	}
	volume := interval(reference[0][0], reference[0][1])
	contains(mass.Volume, volume)
	for axis := range 3 {
		a := new(big.Rat).Add(reference[axis+1][0], new(big.Rat).Mul(rat(shift[axis], 1), reference[0][0]))
		b := new(big.Rat).Add(reference[axis+1][1], new(big.Rat).Mul(rat(shift[axis], 1), reference[0][1]))
		moment := interval(a, b)
		if moment.Upper.Sign() >= 0 || mass.FirstMoments[axis].Upper.Sign() >= 0 {
			t.Fatal("negative moment branch not exercised")
		}
		contains(mass.FirstMoments[axis], moment)
		centroid := geom.ExactParameterInterval{Lower: new(big.Rat).Quo(moment.Lower, volume.Lower), Upper: new(big.Rat).Quo(moment.Upper, volume.Upper)}
		contains(mass.Coordinates[axis], centroid)
		if mass.Coordinates[axis].Upper.Sign() >= 0 {
			t.Fatal("negative centroid branch not exercised")
		}
		if new(big.Rat).Sub(mass.FirstMoments[axis].Upper, mass.FirstMoments[axis].Lower).Cmp(width) > 0 {
			t.Fatal("moment width")
		}
	}
	t.Log("whole proof operations", mass.ExactOperations, "subdivisions", mass.ExactSubdivisions)
}
