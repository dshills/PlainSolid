package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

// The original solid is the sheared sleeve with positive weights that
// change its source parameterization while preserving its model geometry.
func TestAlgebraicGraphRationalStockFlux(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	// Reparameterize the sheared square sleeve along X with
	// x(s)=x0+2s/(1+s). The preceding exact-polynomial fixture has
	// the same geometric boundary and an independent closed-form volume.
	stock := reparameterizedGraphSleeveForFlux(t)
	const z = .5 - 11./64
	planes := [2][3]geom.Vec3{
		{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}},
		{{Z: z}, {X: 1, Z: z}, {Y: 1, Z: z + 1./8}},
	}
	o := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	pair, e := stock.ClipAlgebraicGraphCapPair(ctx, planes, o, RationalGraphShellRegionOptions{})
	if e != nil || len(pair.Seams) != 2 {
		t.Fatal("rational original charts must preserve exact two-plane clip", len(pair.Seams), e)
	}
	opts := geom.ExactCurvePlaneEventOptions{MaxOperations: 20000000, MaxCoefficientBits: 32768, MaxSubdivisions: 8192}
	width := q(1, 1<<16)
	if a, e := pair.BoundVerticalPolynomialVolume(ctx, width, o, opts); !errors.Is(e, geom.ErrUnrepresentable) || a.Lower != nil {
		t.Fatal("legacy polynomial proof must still reject weighted charts", a, e)
	}
	var saved bytes.Buffer
	if e := WriteAlgebraicGraphStockJSON(&saved, pair); e != nil {
		t.Fatal(e)
	}
	pair, b, e := ReadAlgebraicGraphStockJSON(ctx, &saved, JSONLimits{}, o, RationalGraphShellRegionOptions{}, opts)
	if e != nil || b.EulerCharacteristic != 2 || b.BoundaryComponents != 1 {
		t.Fatal("loaded shell must retain the same open notch and one component", b.EulerCharacteristic, e)
	}
	a, e := pair.BoundVerticalVolume(ctx, width, o, opts)
	if e != nil || a.Lower == nil || new(big.Rat).Sub(a.Upper, a.Lower).Cmp(width) > 0 {
		t.Fatal("rational sleeve volume flux", a, e)
	}
	lo := new(big.Rat).Sub(q(697, 960), a.Upper)
	hi := new(big.Rat).Sub(q(697, 960), a.Lower)
	if lo.Sign() <= 0 || lo.Mul(lo, lo).Cmp(q(27, 409600)) > 0 || hi.Mul(hi, hi).Cmp(q(27, 409600)) < 0 {
		t.Fatal("weighted sleeve must enclose independent 697/960−3√3/640", a)
	}
	model, e := pair.PrepareVerticalMaterial(ctx, o, opts)
	if e != nil {
		t.Fatal("weighted stock must retain exact vertical fibers", e)
	}
	line, e := model.IntersectRationalLine(ctx, [3]*big.Rat{q(0, 1), q(89, 64), q(513, 1024)},
		[3]*big.Rat{q(1, 1), q(0, 1), q(0, 1)}, [2]*big.Rat{q(-1, 1), q(4, 1)}, opts)
	if e != nil || len(line.Contacts) != 4 || len(line.Material) != 2 || line.Contacts[2].StockFace < 0 {
		t.Fatal("weighted wall line must retain the original void and wall owner", line, e)
	}
	hit, e := line.Contacts[2].Parameter.SignPolynomial(ctx, []*big.Rat{q(-25, 8), q(0, 1), q(1, 1)}, opts)
	if e != nil || hit.Sign != 0 {
		t.Fatal("source weight must not change the exact bore hit", hit, e)
	}

}

func reparameterizedGraphSleeveForFlux(t *testing.T) Solid {
	t.Helper()
	ctx := context.Background()
	q := big.NewRat
	s := makeWarpedSquareRationalGraphTube(t, 1./8)
	var parts []SewFace
	for _, f := range s.Faces() {
		p, e := exactFaceSupportPatch(f, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		old, _, ok := p.RationalBezierCoefficients(2)
		if !ok {
			t.Fatal("fixture requires a single polynomial chart")
		}
		// Detect which chart coordinate changes X. Replace its
		// homogeneous Bernstein net by a projective parameter map.
		// The increasing map t=2s/(1+s) fixes 0 and 1.
		axis := -1
		if old[1][0][0].Sign() != 0 {
			axis = 0
		}
		if old[0][1][0].Sign() != 0 {
			axis = 1
		}
		r, e := p.ExportExactBezierRecipe()
		if e != nil {
			t.Fatal(e)
		}
		for i := range r.Controls {
			mul := 1
			if axis == 0 {
				mul = 1 << (i / 3)
			}
			if axis == 1 {
				mul = 1 << (i % 3)
			}
			for j, x := range r.Controls[i] {
				v, ok := new(big.Rat).SetString(x)
				if !ok {
					t.Fatal(x)
				}
				r.Controls[i][j] = v.Mul(v, q(int64(mul), 1)).RatString()
			}
		}
		p, e = geom.ImportExactBezierSurfaceRecipe(ctx, r, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		surface, e := geom.NewExactBezierSurface(p)
		if e != nil {
			t.Fatal(e)
		}
		part, e := ExactSplineFaceRectangle(ctx, surface, geom.ParameterRectangle{}, f.Reversed, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		parts = append(parts, part)
	}
	out, _, e := Sew(ctx, parts, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal("exact reparameterized stock", e)
	}
	return out
}
