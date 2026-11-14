package trimmed

import (
	"context"
	"errors"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func crossingFootprintRoofShell(t *testing.T, second bool) (Solid, *geom.BSplineSurface) {
	t.Helper()
	xlo, xhi, ylo, yhi, floor := -.125, 1.125, 0., 1., -1.
	if second {
		xlo, xhi, ylo, yhi, floor = 0, 1, -.125, 1.125, -.5
	}
	xs, ys := [3]float64{xlo, (xlo + xhi) / 2, xhi}, [3]float64{ylo, (ylo + yhi) / 2, yhi}
	z := func(i, j int) float64 {
		v := xs[i] * ys[j] / 64
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
	neighbor, ne := SectionSplinePairAlgebraic(ctx, side, pb, &proposal.Seeds, o.Trace, geom.TubeAlgebraicGraphEndpointOptions{})
	if !errors.Is(ne, intersect.ErrUnresolved) || neighbor.Complete || neighbor.Trace.Coverage.CoversAllContacts {
		t.Fatal("neighbor coverage must remain unproved", neighbor.Trace.Issues, ne)
	}
	result, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if !errors.Is(e, intersect.ErrUnresolved) || result.Complete || len(result.Wires) != 0 {
		t.Fatal("global unresolved neighbor must not assemble", result.Pairs, e)
	}
	if len(result.Pairs) == 0 || result.Pairs[len(result.Pairs)-1] != [2]int{4, 1} {
		t.Fatal("unexpected frontier", result.Pairs, e)
	}
	t.Log("proved roof pair with both unique-owner branches; neighbor iso-plane proposals", len(proposal.Seeds.Attempts), "uncovered candidates", len(neighbor.Trace.Coverage.Unresolved), "global accounted branches", len(result.Branches))
}
