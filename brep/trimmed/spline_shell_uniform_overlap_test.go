package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func crossingFootprintRoofShell(t *testing.T, second bool) (Solid, *geom.BSplineSurface) {
	return crossingFootprintRoofShellShape(t, second, false)
}
func crossingFootprintRoofShellShape(t *testing.T, second, bubble bool) (Solid, *geom.BSplineSurface) {
	t.Helper()
	xlo, xhi, ylo, yhi, floor := -.125, 1.125, 0., 1., -1.
	if second {
		xlo, xhi, ylo, yhi, floor = 0, 1, -.125, 1.125, -.5
	}
	xs, ys := [3]float64{xlo, (xlo + xhi) / 2, xhi}, [3]float64{ylo, (ylo + yhi) / 2, yhi}
	z := func(i, j int) float64 {
		v := xs[i] * ys[j] / 64
		if bubble {
			v = []float64{xlo - xlo*xlo, (xlo+xhi)/2 - xlo*xhi, xhi - xhi*xhi}[i] * []float64{ylo - ylo*ylo, (ylo+yhi)/2 - ylo*yhi, yhi - yhi*yhi}[j] / 64
		}
		if second {
			v += []float64{3.0 / 16, -5.0 / 16, 3.0 / 16}[i] + []float64{ylo - ylo*ylo, (ylo+yhi)/2 - ylo*yhi, yhi - yhi*yhi}[j]/64
		}
		return v
	}
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	add := func(nu, nv int, d geom.ParameterRectangle, rev bool, point func(int, int) geom.Vec3) {
		var poles []geom.Vec3
		for i := 0; i < nu; i++ {
			for j := 0; j < nv; j++ {
				poles = append(poles, point(i, j))
			}
		}
		knots := func(n int, lo, hi float64) []float64 {
			k := make([]float64, 2*n)
			for i := 0; i < n; i++ {
				k[i], k[i+n] = lo, hi
			}
			return k
		}
		p, e := geom.NewBSplineSurface(nu-1, nv-1, nu, nv, poles, knots(nu, d.UMin, d.UMax), knots(nv, d.VMin, d.VMax), nil)
		if e != nil {
			t.Fatal(e)
		}
		surfaces = append(surfaces, p)
		domains = append(domains, d)
		reversed = append(reversed, rev)
	}
	roof := geom.ParameterRectangle{UMin: xlo, UMax: xhi, VMin: ylo, VMax: yhi}
	add(2, 2, roof, true, func(i, j int) geom.Vec3 { return geom.Vec3{X: xs[2*i], Y: ys[2*j], Z: floor} })
	add(3, 3, roof, false, func(i, j int) geom.Vec3 { return geom.Vec3{X: xs[i], Y: ys[j], Z: z(i, j)} })
	for _, edge := range []int{0, 2} {
		add(3, 2, geom.ParameterRectangle{UMin: ylo, UMax: yhi, VMax: 1}, edge == 0, func(i, j int) geom.Vec3 { return geom.Vec3{X: xs[edge], Y: ys[i], Z: []float64{floor, z(edge, i)}[j]} })
	}
	for _, edge := range []int{0, 2} {
		add(3, 2, geom.ParameterRectangle{UMin: xlo, UMax: xhi, VMax: 1}, edge == 2, func(i, j int) geom.Vec3 { return geom.Vec3{X: xs[i], Y: ys[edge], Z: []float64{floor, z(i, edge)}[j]} })
	}
	s, _, e := SewSplineRectangles(context.Background(), surfaces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return s, surfaces[1]
}
func TestAutomaticOverlappingRoofShellContinuation(t *testing.T) {
	ctx := context.Background()
	a, ar := crossingFootprintRoofShell(t, false)
	b, br := crossingFootprintRoofShell(t, true)
	o := AlgebraicShellSectionOptions{}
	o.Trace.UniformOverlapGraphFallback = intersect.UniformGraphDiscoveryOptions{GridDivisions: 16, AxisDivisions: [2]int{8, 16}}
	o.Trace.UniformOverlapGraphFallback.Enabled = true
	o.Trace.UniformOverlapGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.UniformOverlapGraphFallback.MaxArcs = 1024
	o.Trace.UniformOverlapGraphFallback.MaxTubes = 1024
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	o.Trace.Branches.PruneSeparatedEndpoints = true
	o.Trace.Branches.MaxComparisons = 4096
	o.Trace.MaxTopologyPairs = 1000000
	o.Trace.Coverage.MaxCandidateTests = 4096
	o.Trace.Coverage.IsoPlaneFallback = true
	o.Trace.PlaneFallback.Enabled = true
	o.Trace.PlaneFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	pa := intersect.Patch{Surface: ar, Min: geom.Vec2{X: -.125}, Max: geom.Vec2{X: 1.125, Y: 1}}
	pb := intersect.Patch{Surface: br, Min: geom.Vec2{Y: -.125}, Max: geom.Vec2{X: 1, Y: 1.125}}
	for order, pair := range [][2]intersect.Patch{{pa, pb}, {pb, pa}} {
		section, e := SectionSplinePairAlgebraic(ctx, pair[0], pair[1], nil, o.Trace, geom.TubeAlgebraicGraphEndpointOptions{})
		if e != nil || !section.Complete || len(section.Open) != 2 {
			t.Fatal("full original roof pair", len(section.Open), section.Trace.Issues, e)
		}
		for _, branch := range section.Open {
			for _, end := range branch.Ends {
				if end.Operand != order || (end.Side != geom.SurfaceEdgeVMin && end.Side != geom.SurfaceEdgeVMax) {
					t.Fatal("wrong original boundary owner", end.Operand, end.Side)
				}
			}
		}
		missing := intersect.SplineSeedReport{}
		for _, seed := range section.Trace.Seeds.Attempts {
			if seed.Cell.DomainA.UMax < .5 {
				missing.Attempts = append(missing.Attempts, seed)
			}
		}
		bad, e := SectionSplinePairAlgebraic(ctx, pair[0], pair[1], &missing, o.Trace, geom.TubeAlgebraicGraphEndpointOptions{})
		if e == nil || bad.Complete {
			t.Fatal("omitted overlap branch accepted")
		}
	}
	sidePatch, e := exactFaceSupportPatch(a.faces[4], geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	side, _ := algebraicShellTracePatch(a.faces[4], sidePatch.Domain())
	proposal, pe := intersect.DiscoverSplinePlanePairSections(ctx, side, pb, geom.SurfacePatchOptions{}, o.Trace.PlaneFallback)
	if pe != nil || proposal.OpenContours != 1 || len(proposal.Seeds.Attempts) == 0 {
		t.Fatal("neighbor proposal", pe)
	}
	cover, ce := intersect.CertifyIsoPlaneSplineTubeCoverage(ctx, side, pb, proposal.Seeds, intersect.IsoPlaneTubeCoverageOptions{})
	if ce != nil || !cover.Covered || len(cover.Spans) != 1 || cover.Spans[0][0].RatString() != "1/4" || cover.Spans[0][1].RatString() != "3/4" {
		t.Fatal("iso coverage", cover, ce)
	}
	reversed := proposal.Seeds
	reversed.Attempts = append([]intersect.SplineSeedCell(nil), reversed.Attempts...)
	for i := range reversed.Attempts {
		seed := &reversed.Attempts[i]
		seed.Cell.DomainA, seed.Cell.DomainB = seed.Cell.DomainB, seed.Cell.DomainA
		tube := *seed.Tube
		tube.FreeAxis = (tube.FreeAxis + 2) % 4
		seed.Tube = &tube
	}
	reverseCover, re := intersect.CertifyIsoPlaneSplineTubeCoverage(ctx, pb, side, reversed, intersect.IsoPlaneTubeCoverageOptions{})
	if re != nil || !reverseCover.Covered || reverseCover.SourceOperand != 0 {
		t.Fatal("reversed iso owners", re)
	}
	for _, omit := range []int{0, 2, 5} {
		missing := proposal.Seeds
		missing.Attempts = append([]intersect.SplineSeedCell(nil), proposal.Seeds.Attempts[:omit]...)
		missing.Attempts = append(missing.Attempts, proposal.Seeds.Attempts[omit+1:]...)
		bad, e := intersect.CertifyIsoPlaneSplineTubeCoverage(ctx, side, pb, missing, intersect.IsoPlaneTubeCoverageOptions{})
		if e == nil || bad.Covered || len(bad.Spans) > 0 {
			t.Fatal("omitted iso segment", omit, e)
		}
	}
	tiny, e := intersect.CertifyIsoPlaneSplineTubeCoverage(ctx, side, pb, proposal.Seeds, intersect.IsoPlaneTubeCoverageOptions{MaxOperations: 1})
	if e == nil || tiny.Covered || len(tiny.AcceptedTubes) > 0 {
		t.Fatal("atomic iso budget", e)
	}
	near := proposal.Seeds
	near.Attempts = append([]intersect.SplineSeedCell(nil), near.Attempts...)
	near.Attempts[0].Cell.DomainA.VMax = math.Nextafter(1, 0)
	bad, be := intersect.CertifyIsoPlaneSplineTubeCoverage(ctx, side, pb, near, intersect.IsoPlaneTubeCoverageOptions{})
	if be == nil || bad.Covered {
		t.Fatal("one-ULP end gap accepted", be)
	}
	neighbor, ne := SectionSplinePairAlgebraic(ctx, side, pb, &proposal.Seeds, o.Trace, geom.TubeAlgebraicGraphEndpointOptions{})
	if ne != nil || !neighbor.Complete || !neighbor.Trace.Coverage.CoversAllContacts {
		t.Fatal("exact neighbor section failed", neighbor.Trace.Issues, ne)
	}
	result, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if !errors.Is(e, intersect.ErrUnresolved) || result.Complete || len(result.Wires) != 0 {
		t.Fatal("unproved global continuation", e)
	}
	if len(result.Pairs) == 0 || result.Pairs[len(result.Pairs)-1] != [2]int{5, 1} {
		t.Fatal("unexpected frontier", result.Pairs, e)
	}
	t.Log("exact iso neighbor complete; remaining planar-wall frontier", result.Pairs[len(result.Pairs)-1])
}

func TestAutomaticOverlappingBubbleRoofShell(t *testing.T) {
	ctx := context.Background()
	a, _ := crossingFootprintRoofShellShape(t, false, true)
	b, _ := crossingFootprintRoofShellShape(t, true, true)
	o := AlgebraicShellSectionOptions{}
	o.Trace.UniformOverlapGraphFallback = intersect.UniformGraphDiscoveryOptions{GridDivisions: 16, AxisDivisions: [2]int{8, 16}}
	o.Trace.UniformOverlapGraphFallback.Enabled = true
	o.Trace.UniformOverlapGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.UniformOverlapGraphFallback.MaxArcs = 1024
	o.Trace.UniformOverlapGraphFallback.MaxTubes = 1024
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	o.Trace.Branches.PruneSeparatedEndpoints = true
	o.Trace.Branches.MaxComparisons = 4096
	o.Trace.MaxTopologyPairs = 1000000
	o.Trace.Coverage.MaxCandidateTests = 4096
	o.Trace.Coverage.IsoPlaneFallback = true
	o.Trace.PlaneFallback.Enabled = true
	o.Trace.PlaneFallback.Plane.Arcs.ResolveSubdivisionCorners = true

	r, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !r.Complete || len(r.Wires) != 2 || len(r.Branches) != 12 {
		t.Fatal("bubble shell", r.Complete, len(r.Branches), r.Wires, r.Pairs, e)
	}
	seen := map[int]bool{}
	for _, wire := range r.Wires {
		if len(wire) != 4 && len(wire) != 8 {
			t.Fatal("unexpected cycle size", len(wire))
		}
		for i, use := range wire {
			if seen[use.Branch] {
				t.Fatal("duplicate branch incidence")
			}
			seen[use.Branch] = true
			branch := r.Branches[use.Branch]
			if branch.Closed || branch.EdgeParameters[0] == nil || branch.EdgeParameters[1] == nil {
				t.Fatal("lost original edge roots")
			}
			end := 1
			if use.Reversed {
				end = 0
			}
			nextUse := wire[(i+1)%len(wire)]
			next := r.Branches[nextUse.Branch]
			start := 0
			if nextUse.Reversed {
				start = 1
			}
			equal, _, e := branch.Ends[end].Certificate.ModelEvent.CompareModelPoint(ctx, next.Ends[start].Certificate.ModelEvent, geom.ExactCurvePlaneEventOptions{})
			if e != nil || !equal {
				t.Fatal("unmatched original model junction", e)
			}
		}
	}
	if len(seen) != len(r.Branches) {
		t.Fatal("omitted branch")
	}
	for i, pair := range r.Pairs {
		if pair == [2]int{4, 2} {
			seeds := r.Sections[i].Trace.Seeds
			seeds.Attempts = seeds.Attempts[:len(seeds.Attempts)-1]
			ap, _ := exactFaceSupportPatch(a.faces[4], geom.SurfacePatchOptions{})
			bp, _ := exactFaceSupportPatch(b.faces[2], geom.SurfacePatchOptions{})
			pa, _ := algebraicShellTracePatch(a.faces[4], ap.Domain())
			pb, _ := algebraicShellTracePatch(b.faces[2], bp.Domain())
			bad, e := intersect.CertifyIsoPlaneSplineTubeCoverage(ctx, pa, pb, seeds, intersect.IsoPlaneTubeCoverageOptions{})
			if e == nil || bad.Covered {
				t.Fatal("omitted nonbinary terminal tube", e)
			}
		}
	}
	t.Log("two complete original-owner wires", len(r.Wires[0]), len(r.Wires[1]))
}
