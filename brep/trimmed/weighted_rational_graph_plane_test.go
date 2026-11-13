package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestWeightedRationalSplineGraphPlaneClip(t *testing.T) {
	ctx := context.Background()
	var supports []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reverse []bool
	k := []float64{0, 0, 0, 1, 1, 1}
	for a := range 3 {
		u, v := (a+1)%3, (a+2)%3
		for b := range 2 {
			var poles []geom.Vec3
			var weights []float64
			for i := range 3 {
				for j := range 3 {
					idx := [3]int{}
					idx[a], idx[u], idx[v] = 2*b, i, j
					x := [...]float64{0, 2, 3}[idx[0]]
					w := [...]float64{1, .75, .5}[idx[0]]
					y, z := float64(idx[1])/2, float64(idx[2])/2
					z += y / 4
					if idx[0] == 2 {
						z += 1. / 16
					}
					poles = append(poles, geom.Vec3{X: x, Y: y, Z: z})
					weights = append(weights, w)
				}
			}
			s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, weights)
			if e != nil {
				t.Fatal(e)
			}
			supports = append(supports, s)
			domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
			reverse = append(reverse, b == 0)
		}
	}
	stock, _, e := SewSplineRectangles(ctx, supports, domains, reverse, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{7, 3, 3. / 16}, 0)
	r, e := CombineRationalGraphSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{})
	if e != nil || len(r.SectionEdges) != 4 || len(r.Solid.Topology().Shells) != 1 {
		t.Fatal("weighted stock graph clip", e)
	}
	bottom := r.Solid.faces[r.StockFaces[4][0]]
	if bottom.Surface != supports[4] {
		t.Fatal("replaced original rational support")
	}
	found := false
	for _, use := range bottom.Outer {
		if q, ok := use.Curve.(ExactQuadraticUV); ok {
			uv, _, e := q.EvaluateRational(big.NewRat(3, 7))
			if e != nil {
				t.Fatal(e)
			}
			s, y := uv[0], uv[1]
			a := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Quo(s, big.NewRat(2, 1)))
			f := new(big.Rat).Mul(new(big.Rat).Sub(big.NewRat(6, 1), new(big.Rat).Mul(y, big.NewRat(8, 1))), a)
			if f.Cmp(new(big.Rat).Mul(s, s)) != 0 {
				t.Fatal("exact weighted UV residual", uv)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing weighted section on stock support")
	}
	plane := [3]geom.Vec3{{Z: 3. / 16}, {X: 1, Z: 13. / 64}, {Y: 1, Z: 3. / 16}}
	section, e := CertifyRationalGraphPlaneFaceSection(ctx, stock, 4, plane, 0, PolynomialPlaneSpanOptions{})
	if e != nil || section.Face() != 4 || section.FreeAxis() != 0 || section.PlaneSideSign() != 1 || len(section.Pieces()) != 1 || section.ProjectionAxes() != [2]int{0, 1} {
		t.Fatal("missing exact oblique face section", section, e)
	}
	sectionFaces := stock.faces[4]
	for i, id := range section.Edges() {
		p := section.Parameters()[i]
		if p.Cmp(big.NewRat(0, 1)) <= 0 || p.Cmp(big.NewRat(1, 1)) >= 0 {
			t.Fatal("source edge range not retained", id, p)
		}
		found = false
		for _, u := range sectionFaces.Outer {
			found = found || u.Edge == id
		}
		if !found {
			t.Fatal("edge is not owned by original face")
		}
	}
	cells, e := section.ModelCurveCells(ctx, 0, geom.SurfaceCurveAgreementOptions{})
	if e != nil || !cells.Complete || len(cells.Cells) != 1 {
		t.Fatal("section lost original support composition", cells, e)
	}
	copyPieces := section.Pieces()
	copyPieces[0] = nil
	if section.Pieces()[0] == nil {
		t.Fatal("section allowed modification of its trim list")
	}
	checkSection := func(plane [3]geom.Vec3, orientation int) RationalGraphShellSection {
		t.Helper()
		w, e := stock.SectionRationalGraphSplineShell(ctx, plane, RationalGraphShellSectionOptions{VerifyModelEdges: true, Regions: &RationalGraphShellRegionOptions{}})
		if e != nil || !w.Complete || len(w.Wires) != 1 || len(w.Wires[0]) != 4 || len(w.Branches) != 4 {
			t.Fatal("entire oblique shell section", w, e)
		}
		axes := [2]int{0, 1}
		if orientation < 0 {
			axes = [2]int{1, 0}
		}
		if w.Regions.ProjectionAxes != axes || len(w.Regions.Material) != 1 || w.Regions.Material[0].Outer != 0 {
			t.Fatal("wrong oriented planar material region", w.Regions)
		}
		wire := w.Wires[0]
		found := false
		for i, id := range wire {
			b := w.Branches[id]
			next := w.Branches[wire[(i+1)%len(wire)]]
			start, end := b.Ends[0], b.Ends[1]
			if end.Edge != next.Ends[0].Edge || end.Parameter.Cmp(next.Ends[0].Parameter) != 0 {
				t.Fatal("original model edge parameter was not retained")
			}
			for k := range end.Point {
				if end.Point[k].Cmp(next.Ends[0].Point[k]) != 0 {
					t.Fatal("exact model terminal mismatch")
				}
			}
			if b.Face != 4 {
				continue
			}
			found = true
			// Bottom-face outward normal has negative Z. For this plane,
			// the positive wire traverses the bottom branch toward lower X.
			if end.Point[0].Cmp(start.Point[0]) != -orientation || len(b.Pieces) != 1 {
				t.Fatal("incorrect original-chart traversal", b)
			}
			piece, ok := b.Pieces[0].(ExactQuadraticUV)
			if !ok {
				t.Fatal("missing exact rational trim")
			}
			uv, _, e := piece.EvaluateRational(big.NewRat(3, 7))
			if e != nil {
				t.Fatal(e)
			}
			p, e := section.proof.patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
			if e != nil || new(big.Rat).Sub(p.Point[2], new(big.Rat).Quo(p.Point[0], big.NewRat(64, 1))).Cmp(big.NewRat(3, 16)) != 0 {
				t.Fatal("retained curved trim is off the authored plane", p, e)
			}
		}
		if !found {
			t.Fatal("missing original curved face")
		}
		return w
	}
	w := checkSection(plane, 1)
	area, e := w.Regions.BoundArea(ctx, big.NewRat(1, 10000))
	if e != nil || area.ModelScaleSquared.Cmp(big.NewRat(4097, 4096)) != 0 {
		t.Fatal("wrong area scale on oblique slice", area, e)
	}
	// In projected coordinates X=3*s. The bottom crossing lies at
	// y=3/4+3*s/16-s²/(8(1-s/2)), above y=0 and below y=1.
	// The integral of s²/(1-s/2) over [0,1] is 8 ln 2 - 5.
	expect := 3 * (3./4 + 3./32 - (8*math.Log(2)-5)/8)
	l, _ := area.Projection.Lower.Float64()
	u, _ := area.Projection.Upper.Float64()
	if l > expect || u < expect || u-l > .0001 {
		t.Fatal("projected rational area doesn't enclose the independent integral", l, u, expect)
	}
	capSupport, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{
		{Z: 3. / 16}, {Y: 1, Z: 3. / 16}, {X: 4, Z: 1. / 4}, {X: 4, Y: 1, Z: 1. / 4},
	}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	domain := geom.ParameterRectangle{UMax: 1, VMax: 1}
	caps, e := w.Regions.BuildCaps(ctx, capSupport, domain, RationalGraphSectionCapOptions{})
	if e != nil || len(caps) != 1 || len(caps[0].Edges) != 4 || caps[0].Boundary.Face.Reversed {
		t.Fatal("oblique cap reconstruction", caps, e)
	}
	cp, e := geom.NewSurfacePatch(capSupport, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, source := range caps[0].Edges {
		if source.Face != 4 {
			continue
		}
		t0 := big.NewRat(2, 7)
		a, _, e1 := w.Branches[source.Branch].Pieces[source.Piece].(ExactQuadraticUV).EvaluateRational(t0)
		b, _, e2 := caps[0].Boundary.Face.Outer[i].Curve.(ExactQuadraticUV).EvaluateRational(t0)
		if e1 != nil || e2 != nil {
			t.Fatal(e1, e2)
		}
		x, e1 := section.proof.patch.EvaluateRational(ctx, a[0], a[1], geom.SurfacePatchEvaluationOptions{})
		y, e2 := cp.EvaluateRational(ctx, b[0], b[1], geom.SurfacePatchEvaluationOptions{})
		if e1 != nil || e2 != nil {
			t.Fatal(e1, e2)
		}
		for j := range x.Point {
			if x.Point[j].Cmp(y.Point[j]) != 0 {
				t.Fatal("cap transport broke exact original support identity", x.Point, y.Point)
			}
		}
	}
	if bad, e := w.Regions.BuildCaps(ctx, capSupport, geom.ParameterRectangle{UMax: .5, VMax: 1}, RationalGraphSectionCapOptions{}); !errors.Is(e, geom.ErrDomain) || bad != nil {
		t.Fatal("clip to undersized chart exposed a partial cap", bad, e)
	}
	split, e := stock.SplitRationalGraphSplineShell(ctx, plane, capSupport, domain, RationalGraphShellSplitOptions{
		Section: RationalGraphShellSectionOptions{VerifyModelEdges: true},
	})
	if e != nil || !split.Section.Complete {
		t.Fatal("whole curved shell split", e)
	}
	const nSplit = 512
	var expectedNegative float64
	for i := 0; i <= nSplit; i++ {
		s := float64(i) / nSplit
		// Cut z=3/16+3s/64. The lower stock is y/4+s²/(32(1−s/2)).
		// For fixed s the positive thickness is a triangle in the y/z slice.
		h := 3./16 + 3*s/64 - s*s/(32*(1-s/2))
		weight := 2.
		if i == 0 || i == nSplit {
			weight = 1
		} else if i%2 != 0 {
			weight = 4
		}
		expectedNegative += weight * 6 * h * h / (3 * nSplit)
	}
	for side, h := range split.Halves {
		if len(h.CapFaces) != 1 || len(h.CapEdges[0]) != 4 || len(h.Solid.Topology().Shells) != 1 {
			t.Fatal("curved split boundaries are not individually closed", side)
		}
		faces := h.Solid.Faces()
		if faces[h.CapFaces[0]].Reversed != (side == 1) || faces[h.SourceFaces[4][0]].Surface != supports[4] {
			t.Fatal("lost outward cap orientation or original support", side)
		}
		properties, e := h.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
		want := expectedNegative
		if side == 1 {
			want = 3 - want
		}
		if e != nil || math.Abs(properties.Volume-want) > 1e-7 {
			t.Fatal("independent split volume", side, properties.Volume, want, e)
		}
	}
	if bad, e := stock.SectionRationalGraphSplineShell(ctx, plane, RationalGraphShellSectionOptions{MaxPieces: len(w.Branches) - 1}); e == nil || bad.Complete || len(bad.Branches) != 0 || len(bad.Wires) != 0 {
		t.Fatal("budget exposed a partial section", bad, e)
	}
	plane[1], plane[2] = plane[2], plane[1]
	negative, e := CertifyRationalGraphPlaneFaceSection(ctx, stock, 4, plane, 0, PolynomialPlaneSpanOptions{})
	if e != nil || negative.PlaneSideSign() != -1 || negative.Edges() != section.Edges() {
		t.Fatal("reversed plane lost ownership or orientation", e)
	}
	checkSection(plane, -1)
	// Independent vertical wedge: X=3*s, lower height is s²/(32*(1-s/2))+Y/4.
	// Integrate Y first and then the smooth one-dimensional section by Simpson.
	const n = 1024
	var volume float64
	for i := 0; i <= n; i++ {
		s := float64(i) / n
		h := 3./16 - s*s/(32*(1-s/2))
		w := 2 + 2*(i%2)
		if i == 0 || i == n {
			w = 1
		}
		volume += 6 * h * h * float64(w) / (3 * n)
	}
	props, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(props.Volume-volume) > 1e-8 {
		t.Fatal("independent weighted wedge volume", props.Volume, volume, e)
	}
	for _, variant := range []string{"dependent weight", "nonaffine projection"} {
		d := supports[4].Data()
		if variant == "dependent weight" {
			d.Weights[4] *= .5
		} else {
			d.Poles[4].X += 1. / 32
		}
		s, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		q, _, e := rationalGraphPlaneTrim(p, horizontalPolynomialPlane(big.NewRat(3, 16)), 0)
		if q != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatalf("accepted %s: %v", variant, e)
		}
	}
}
