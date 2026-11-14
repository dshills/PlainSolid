package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"os/exec"
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

	mass, e := pair.BoundVerticalCentroid(ctx, q(1, 1<<15), o, opts)
	if e != nil || mass.Volume.Lower.Sign() <= 0 {
		t.Fatal("weighted original mass and centroid", e)
	}
	// Integrate over vertical slices with y between max(11/8,x²/8)
	// and 3+x²/8, subtracting the old bore on 1<=x<=2. The bore
	// interval changes its lower endpoint at x=√3.
	ref := [4][2]*big.Rat{{q(697, 960), q(3, 640)}, {q(7939, 6144), q(0, 1)},
		{q(7349, 3584), q(39, 7168)}, {q(184411, 430080), q(327, 143360)}}
	got := [4]geom.ExactParameterInterval{mass.Volume, mass.FirstMoments[0], mass.FirstMoments[1], mass.FirstMoments[2]}
	for i, m := range got {
		v, k := ref[i][0], ref[i][1]
		if k.Sign() == 0 {
			if m.Lower.Cmp(v) > 0 || m.Upper.Cmp(v) < 0 {
				t.Fatal("closed-hand formula for x moment", m)
			}
			continue
		}
		lo, hi := new(big.Rat).Sub(v, m.Upper), new(big.Rat).Sub(v, m.Lower)
		want := new(big.Rat).Mul(q(3, 1), new(big.Rat).Mul(k, k))
		if lo.Sign() <= 0 || lo.Mul(lo, lo).Cmp(want) > 0 || hi.Mul(hi, hi).Cmp(want) < 0 {
			t.Fatal("moment does not contain its distinct independent algebraic answer", i, m)
		}
	}
	// Independently calculated coordinate checks avoid quotient values
	// outside the bounding box being hidden by a wide integral interval.
	for i, want := range [3]float64{1.7998525281375826, 2.843033752390164, .5917521095243853} {
		c := mass.Coordinates[i]
		lo, _ := c.Lower.Float64()
		hi, _ := c.Upper.Float64()
		if hi-lo > .0002 || want < lo || want > hi {
			t.Fatal("weighted centroid", i, lo, hi, want)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ALGEBRAIC_CAP_STOCK"); oracle != "" {
		data, e := exec.CommandContext(ctx, oracle, "sheared-sleeve").Output()
		var answer [8]float64
		if e != nil || json.Unmarshal(data, &answer) != nil {
			t.Fatal("independent OCCT centroid", e)
		}
		for i, b := range mass.Coordinates {
			v := ratInset(answer[i+2])
			if v.Cmp(b.Lower) < 0 || v.Cmp(b.Upper) > 0 {
				t.Fatal("OCCT centroid outside certified interval", i, answer[i+2])
			}
		}
	}
	if out, e := pair.BoundVerticalCentroid(ctx, q(1, 1<<15), o, geom.ExactCurvePlaneEventOptions{MaxOperations: 1}); !errors.Is(e, geom.ErrConvergence) || out.Volume.Lower != nil {
		t.Fatal("centroid budget must fail atomically", out, e)
	}

	opts.MaxOperations = mass.ExactOperations
	if out, e := pair.BoundVerticalCentroid(ctx, q(1, 1<<15), o, opts); !errors.Is(e, geom.ErrConvergence) || out.Volume.Lower != nil {
		t.Fatal("the reported work exhausts exactly one shared proof ledger", out, e)
	}

	opts.MaxOperations = 20000000
	tensor, e := pair.BoundVerticalMass(ctx, q(1, 1<<13), o, opts)
	if e != nil {
		t.Fatal("weighted sleeve raw moments and centroid inertia", e)
	}
	rr := [6][2]*big.Rat{
		{q(19441, 6720), q(9, 4480)}, {q(7762633, 1290240), q(1821, 286720)},
		{q(7939259, 30965760), q(2547, 2293760)}, {q(375089, 98304), q(0, 1)},
		{q(405541, 524288), q(0, 1)}, {q(12644999, 10321920), q(3039, 1146880)},
	}
	for i, u := range tensor.SecondMoments {
		v, k := rr[i][0], rr[i][1]
		if k.Sign() == 0 {
			if u.Lower.Cmp(v) > 0 || u.Upper.Cmp(v) < 0 {
				t.Fatal("raw second mixed moment", i, u)
			}
			continue
		}
		lo, hi := new(big.Rat).Sub(v, u.Upper), new(big.Rat).Sub(v, u.Lower)
		want := new(big.Rat).Mul(q(3, 1), new(big.Rat).Mul(k, k))
		if lo.Sign() <= 0 || lo.Mul(lo, lo).Cmp(want) > 0 || hi.Mul(hi, hi).Cmp(want) < 0 {
			t.Fatal("independent raw second moment", i, u)
		}
	}
	// Calculate the reference inertia from independent vertical slices,
	// direct centroid reduction and Cartesian tensor convention.
	m2 := [6]float64{2.8895263860264615, 6.005425122192287, .2544650381262126, 3.8156026204427085, .7735080718994141, 1.2204730978696003}
	m1 := [3]float64{1.2921549479166667, 2.041078406599444, .42483225948143355}
	v := .7179226785061876
	for i, ij := range [6][2]int{{0, 0}, {1, 1}, {2, 2}, {0, 1}, {0, 2}, {1, 2}} {
		m2[i] -= m1[ij[0]] * m1[ij[1]] / v
	}
	for i, ij := range [6][2]int{{0, 0}, {1, 1}, {2, 2}, {0, 1}, {0, 2}, {1, 2}} {
		want := -m2[i]
		if i < 3 {
			want = m2[(i+1)%3] + m2[(i+2)%3]
		}
		a := tensor.CentroidInertia[ij[0]][ij[1]]
		lo, _ := a.Lower.Float64()
		hi, _ := a.Upper.Float64()
		z := tensor.CentroidInertia[ij[1]][ij[0]]
		if lo > want || hi < want || hi-lo > .003 || z.Lower.Cmp(a.Lower) != 0 || z.Upper.Cmp(a.Upper) != 0 {
			t.Fatal("inertia bound or Cartesian tensor symmetry", ij, lo, hi, want)
		}
	}

	if oracle := os.Getenv("PLAINSOLID_OCCT_ALGEBRAIC_CAP_INERTIA"); oracle != "" {
		data, e := exec.CommandContext(ctx, oracle, "sheared-sleeve-inertia").Output()
		var tensorRef [9]float64
		if e != nil || json.Unmarshal(data, &tensorRef) != nil {
			t.Fatal("independent OCCT inertia tensor", e)
		}
		for i, row := range tensor.CentroidInertia {
			for j, cell := range row {
				v := ratInset(tensorRef[3*i+j])
				if cell.Lower.Cmp(v) > 0 || cell.Upper.Cmp(v) < 0 {
					t.Fatal("independent OCCT tensor outside the certificate", i, j, tensorRef[3*i+j])
				}
			}
		}
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
