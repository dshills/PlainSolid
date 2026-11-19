//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestRationalGraphPolygonalCapCutThroughHole(t *testing.T) {
	ctx := context.Background()
	stock := makeSquareRationalGraphTube(t)
	sec, e := stock.SectionRationalGraphSplineShell(ctx, [3]geom.Vec3{{Z: .5}, {X: 1, Z: .5}, {Y: 1, Z: .5}}, RationalGraphShellSectionOptions{Regions: &RationalGraphShellRegionOptions{}})
	if e != nil {
		t.Fatal(e)
	}
	cut := func(x float64) [3]geom.Vec3 { return [3]geom.Vec3{{X: x}, {X: x, Y: 1}, {X: x, Z: 1}} }
	for _, keep := range []bool{true, false} {
		r, e := sec.Regions.IntersectPolygonalHalfspace(ctx, cut(1.5), keep, RationalGraphShellRegionOptions{})
		if e != nil {
			t.Fatal(e)
		}
		algebraic, e := sec.Regions.SectionAlgebraicHalfspace(ctx, cut(1.5), keep, RationalGraphShellRegionOptions{})
		if e != nil || len(algebraic.Contacts) != 4 || len(algebraic.Material) != 2 {
			t.Fatal("algebraic order must exclude the same through-void interval", algebraic, e)
		}
		clipped, e := sec.Regions.ClipAlgebraicHalfspace(ctx, cut(1.5), keep, RationalGraphShellRegionOptions{})
		if e != nil || len(clipped.Wires) != 1 || len(clipped.Section.Material) != 2 {
			t.Fatal("rejoin both original rings into one notched wire", clipped, e)
		}
		counts := map[int]int{}
		for _, use := range clipped.Wires[0] {
			counts[use.InputWire]++
		}
		if counts[-1] != 2 || counts[0] < 3 || counts[1] < 3 {
			t.Fatal("notch must include both original loops connected by two seams", counts)
		}
		checkAlgebraicGraphCapClosure(t, clipped)
		if len(r.Cuts) != 4 || len(r.Seams) != 2 || len(r.Regions.Material) != 1 || len(r.Regions.Material[0].Holes) != 0 {
			t.Fatal("inner loop must become a notch with two disjoint cap seams", r)
		}
		area, e := r.Regions.BoundArea(ctx, big.NewRat(1, 1024))
		if e != nil || area.Projection.Lower.Cmp(big.NewRat(4, 1)) != 0 || area.Projection.Upper.Cmp(area.Projection.Lower) != 0 {
			t.Fatal("the exact symmetric half of the original annulus has area 4", area, e)
		}
		var ends [4]bool
		for _, seam := range r.Seams {
			a, b := r.Cuts[seam.From], r.Cuts[seam.To]
			if !a.KeepBefore || b.KeepBefore || a.Point[0].Cmp(big.NewRat(3, 2)) != 0 || b.Point[0].Cmp(big.NewRat(3, 2)) != 0 ||
				new(big.Rat).Abs(new(big.Rat).Sub(a.Point[1], b.Point[1])).Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal("generated segment must span material, with original boundary ancestry", a, b)
			}
			for _, endpoint := range []int{seam.From, seam.To} {
				y := r.Cuts[endpoint].Point[1]
				if !y.IsInt() || y.Sign() < 0 || y.Cmp(big.NewRat(3, 1)) > 0 || ends[y.Num().Int64()] {
					t.Fatal("ambiguous terminal", endpoint, y)
				}
				ends[y.Num().Int64()] = true
			}
		}
		for _, q := range []struct {
			y float64
			l trimvalidate.Location
		}{{.5, trimvalidate.Boundary}, {1.5, trimvalidate.Outside}, {2.5, trimvalidate.Boundary}} {
			l, p, e := r.Regions.LocateRational(ctx, [3]*big.Rat{big.NewRat(3, 2), ratInset(q.y), big.NewRat(1, 2)})
			if e != nil || p.Status != trimvalidate.Certified || l != q.l {
				t.Fatal("line ownership across the removed hole", q, l, p, e)
			}
		}
	}
	if out, e := sec.Regions.IntersectPolygonalHalfspace(ctx, cut(1), true, RationalGraphShellRegionOptions{}); !errors.Is(e, geom.ErrConvergence) || out.Regions != nil || len(out.Cuts)+len(out.Seams) != 0 {
		t.Fatal("original vertex/coincident section must reject without partial output", e)
	}
	if out, e := sec.Regions.IntersectPolygonalHalfspace(ctx, cut(1.5), true, RationalGraphShellRegionOptions{Topology: trimvalidate.Options{MaxWork: 1}}); !errors.Is(e, geom.ErrConvergence) || out.Regions != nil {
		t.Fatal("budget exhaustion must reject atomically", e)
	}
}

func TestRationalGraphCapTwoRootsOnOneEdge(t *testing.T) {
	ctx := context.Background()
	p := func(x, y float64) [3]*big.Rat { return [3]*big.Rat{ratInset(x), ratInset(y), big.NewRat(1, 1)} }
	// CCW outer region, under y=1+4t(1−t), swept from x=4 to x=0.
	a, b, c, d := p(0, 0), p(4, 0), p(4, 1), p(0, 1)
	controls := [][][3]*big.Rat{{a, p(2, 0), b}, {b, p(4, .5), c}, {c, p(2, 3), d}, {d, p(0, .5), a}}
	var wire []projectedGraphSectionPiece
	var ring trimvalidate.Loop
	for i, h := range controls {
		q, e := trimvalidate.ExactBezier(h)
		if e != nil {
			t.Fatal(e)
		}
		wire = append(wire, projectedGraphSectionPiece{source: RationalGraphCapEdgeSource{Branch: i, Face: i}, control: h})
		ring = append(ring, q)
	}
	opts, _ := (trimvalidate.Options{}).WithDefaults()
	base, e := certifyProjectedRationalGraphRegions(ctx, [][]projectedGraphSectionPiece{wire}, []trimvalidate.Loop{ring}, horizontalPolynomialPlane(new(big.Rat)), opts)
	if e != nil {
		t.Fatal(e)
	}
	cut := func(y float64) [3]geom.Vec3 { return [3]geom.Vec3{{Y: y}, {Y: y, Z: 1}, {X: 1, Y: y}} }
	r, e := base.IntersectRationalHalfspace(ctx, cut(1.75), true, RationalGraphShellRegionOptions{})
	if e != nil || len(r.Cuts) != 2 || len(r.Seams) != 1 {
		t.Fatal("an edge with equally signed endpoints can cross twice", r, e)
	}
	for i, c := range r.Cuts {
		if c.Source.Branch != 2 || c.Parameter.Cmp(big.NewRat(int64(1+2*i), 4)) != 0 || c.KeepBefore != (i == 1) {
			t.Fatal("wrong exact retained original interval", i, c)
		}
	}
	area, e := r.Regions.BoundArea(ctx, big.NewRat(1, 4096))
	want := big.NewRat(1, 3) // integral of 1/4-(x-2)²/4 over [1,3].
	if e != nil || area.Projection.Lower.Cmp(want) > 0 || area.Projection.Upper.Cmp(want) < 0 {
		t.Fatal("cut cap should be an exact parabolic segment", want, area, e)
	}
	for _, y := range []float64{2, 1.5} {
		if out, e := base.IntersectRationalHalfspace(ctx, cut(y), true, RationalGraphShellRegionOptions{}); !errors.Is(e, geom.ErrConvergence) || out.Regions != nil {
			t.Fatal("tangent and irrational contacts must reject atomically", y, e)
		}
	}
	for _, keep := range []bool{false, true} {
		a, e := base.SectionAlgebraicHalfspace(ctx, cut(1.5), keep, RationalGraphShellRegionOptions{})
		if e != nil || len(a.Material) != 1 || len(a.Contacts) != 2 || a.Material[0] != (RationalGraphCapSeam{0, 1}) {
			t.Fatal("irrational interval must remain exact and complete", a, e)
		}
		for _, c := range a.Contacts {
			if c.Source.Branch != 2 {
				t.Fatal("lost original parameter owner", c.Source)
			}
			q, e := c.Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
			if e != nil || q.Kind != geom.ExactRootKindIrrational {
				t.Fatal("parameter root must retain irrational identity", q, e)
			}
			q, e = c.CoordinateRoots[1].Classify(ctx, geom.ExactCurvePlaneEventOptions{})
			if e != nil || q.Kind != geom.ExactRootKindRational || q.Rational.Cmp(big.NewRat(3, 2)) != 0 {
				t.Fatal("the other coordinate lies exactly on the cut", q, e)
			}
		}
		lo, hi := a.Contacts[0].CoordinateRoots[0], a.Contacts[1].CoordinateRoots[0]
		order, e := lo.Compare(ctx, hi, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !order.Proven || order.Order != map[bool]int{true: -1, false: 1}[keep] {
			t.Fatal("clip orientation must reverse the proved algebraic order", keep, order, e)
		}
		if a.ExactOperations <= 0 || a.Subdivisions <= 0 {
			t.Fatal("algebraic cuts must record charged work", a)
		}
		clip, e := base.ClipAlgebraicHalfspace(ctx, cut(1.5), keep, RationalGraphShellRegionOptions{})
		wantUses := 6 // the original four sides, one split original, and the seam
		if keep {
			wantUses = 2 // just the middle interval of the original arc and the new seam
		}
		if e != nil || len(clip.Wires) != 1 || len(clip.Wires[0]) != wantUses {
			t.Fatal("incorrect kept runs", keep, clip, e)
		}
		checkAlgebraicGraphCapClosure(t, clip)
		width := big.NewRat(1, 1<<28)
		p, e := clip.BoundPolynomialArea(ctx, width, RationalGraphShellRegionOptions{})
		if e != nil || p.Projection.Lower.Sign() <= 0 || p.ModelScaleSquared.Cmp(big.NewRat(1, 1)) != 0 ||
			new(big.Rat).Sub(p.Projection.Upper, p.Projection.Lower).Cmp(width) > 0 {
			t.Fatal("certified projected area", p, e)
		}
		// At x=2±sqrt(2) the parabola meets y=3/2. The positive
		// segment has area 2sqrt(2)/3. The original material area is 20/3.
		areaLo, areaHi := p.Projection.Lower, p.Projection.Upper
		if !keep {
			areaLo, areaHi = new(big.Rat).Sub(big.NewRat(20, 3), areaHi), new(big.Rat).Sub(big.NewRat(20, 3), areaLo)
		}
		square := func(q *big.Rat) *big.Rat { return new(big.Rat).Mul(q, q) }
		if areaLo.Sign() <= 0 || square(areaLo).Cmp(big.NewRat(8, 9)) > 0 || square(areaHi).Cmp(big.NewRat(8, 9)) < 0 {
			t.Fatal("analytic parabolic segment not enclosed", areaLo, areaHi)
		}
		budget := RationalGraphShellRegionOptions{Topology: trimvalidate.Options{MaxWork: 1}}
		if none, e := clip.BoundPolynomialArea(ctx, width, budget); !errors.Is(e, geom.ErrConvergence) || none.Projection.Lower != nil {
			t.Fatal("area budget must not publish partial data", e)
		}
		var sourceUses, seams int
		for _, u := range clip.Wires[0] {
			if u.Seam >= 0 {
				seams++
			}
			if u.InputPiece == 2 {
				sourceUses++
				for j, id := range u.EndpointContacts {
					if id >= 0 {
						p, e := u.SourceParameters[j].Compare(ctx, a.Contacts[id].Parameter, geom.ExactCurvePlaneEventOptions{})
						if e != nil || !p.Proven || p.Order != 0 {
							t.Fatal("cut changed the exact original parameter", p, e)
						}
					}
				}
			}
		}
		if seams != 1 || sourceUses != map[bool]int{true: 1, false: 2}[keep] {
			t.Fatal("must retain the original curved piece on all local intervals", sourceUses, seams)
		}
		clippingLimit := RationalGraphShellRegionOptions{Topology: trimvalidate.Options{MaxWork: clip.ExactOperations}}
		if partial, e := base.ClipAlgebraicHalfspace(ctx, cut(1.5), keep, clippingLimit); !errors.Is(e, geom.ErrConvergence) || len(partial.Wires)+len(partial.Section.Contacts) != 0 {
			t.Fatal("a completed section alone must not publish incomplete clipping", partial, e)
		}
		limit := RationalGraphShellRegionOptions{Topology: trimvalidate.Options{MaxWork: a.ExactOperations - 1}}
		if bad, e := base.SectionAlgebraicHalfspace(ctx, cut(1.5), keep, limit); !errors.Is(e, geom.ErrConvergence) || len(bad.Contacts) != 0 {
			t.Fatal("exhausted algebraic cut must reject atomically", bad, e)
		}
	}
	if out, e := base.SectionAlgebraicHalfspace(ctx, cut(2), true, RationalGraphShellRegionOptions{}); !errors.Is(e, geom.ErrConvergence) || len(out.Contacts)+len(out.Material) != 0 {
		t.Fatal("tangent algebraic cut must not publish contacts", e)
	}
	// Clip the already restricted cap again. Its parabolic owner has a
	// local root at 1/4; the original graph root must be reported as 3/8.
	second := [3]geom.Vec3{{X: 2.5}, {X: 2.5, Y: 1}, {X: 2.5, Z: 1}}
	for i := range r.Cuts {
		r.Cuts[i].Parameter.SetInt64(42)
		for _, x := range r.Cuts[i].Point {
			x.SetInt64(42)
		}
	}
	for _, row := range r.Regions.SectionWires() {
		for _, piece := range row {
			piece.Parameter[0].SetInt64(42)
			piece.Homogeneous[0][0].SetInt64(42)
		}
	}
	s, e := r.Regions.IntersectRationalHalfspace(ctx, second, true, RationalGraphShellRegionOptions{})
	if e != nil || len(s.Cuts) != 2 || len(s.Seams) != 1 {
		t.Fatal("second clip with caller-mutated prior report", s, e)
	}
	for _, c := range s.Cuts {
		if c.Source.Branch == 2 {
			if c.Parameter.Cmp(big.NewRat(3, 8)) != 0 || c.Point[1].Cmp(big.NewRat(31, 16)) != 0 {
				t.Fatal("second clip must retain original parameter and model point", c)
			}
			return
		}
	}
	t.Fatal("missing original curved owner after repeated clipping", s.Cuts)
}

func checkAlgebraicGraphCapClosure(t *testing.T, clip AlgebraicGraphCapClip) {
	t.Helper()
	ctx := context.Background()
	o := geom.ExactCurvePlaneEventOptions{}
	for _, wire := range clip.Wires {
		event := func(u AlgebraicGraphCapUse, side int) *geom.ExactComposedCurveEvent {
			t.Helper()
			if u.Reversed {
				side = 1 - side
			}
			p, e := u.Cell.EndpointRoot(side)
			if e != nil {
				t.Fatal(e)
			}
			x, e := u.Cell.BindRoot(ctx, p, o)
			if e != nil {
				t.Fatal(e)
			}
			return x
		}
		for i, u := range wire {
			v := wire[(i+1)%len(wire)]
			if u.EndpointContacts[1] != v.EndpointContacts[0] {
				t.Fatal("boundary contact incidence", u, v)
			}
			equal, _, e := event(u, 1).CompareModelPoint(ctx, event(v, 0), o)
			if e != nil || !equal {
				t.Fatal("oriented wire endpoints must be the identical model point", i, e)
			}
		}
	}
}
