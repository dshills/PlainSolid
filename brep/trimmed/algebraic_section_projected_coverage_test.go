package trimmed

import (
	"context"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestSectionTwoCurvedBranchesProjectedCoverage(t *testing.T) {
	if testing.Short() {
		t.Skip("complete automatic two-branch original section")
	}
	ctx := context.Background()
	_, a := crossingFootprintRoofShell(t, false)
	_, b := crossingFootprintRoofShell(t, true)
	pa := intersect.Patch{Surface: a, Min: geom.Vec2{X: -.125}, Max: geom.Vec2{X: 1.125, Y: 1}}
	pb := intersect.Patch{Surface: b, Min: geom.Vec2{Y: -.125}, Max: geom.Vec2{X: 1, Y: 1.125}}
	o := intersect.SplinePairTraceOptions{}
	o.UniformOverlapGraphFallback = intersect.UniformGraphDiscoveryOptions{GridDivisions: 16, AxisDivisions: [2]int{8, 16}}
	o.UniformOverlapGraphFallback.Enabled = true
	o.UniformOverlapGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.UniformOverlapGraphFallback.MaxArcs = 1024
	o.UniformOverlapGraphFallback.MaxTubes = 1024
	o.Coverage.Isolation.ParameterTolerance = .125
	o.Coverage.Cell.MaxOperations = 16000000
	o.Branches.PruneSeparatedEndpoints = true
	o.Branches.MaxComparisons = 4096
	o.MaxTopologyPairs = 1000000
	o.Coverage.MaxCandidateTests = 4096
	o.Coverage.CommonXYProjected = &intersect.CommonXYProjectedCoverageOptions{MaxOperations: 128000000}
	section, e := SectionSplinePairAlgebraic(ctx, pa, pb, nil, o, geom.TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !section.Complete || len(section.Open) != 2 {
		t.Fatal("complete original section", section.Trace.Issues, e)
	}
	report := section.Trace.Coverage.CommonXYProjected
	if report == nil || len(report.Projected) == 0 || report.ExactOperations <= 0 {
		t.Fatal("missing selected proof", report)
	}
	for _, branch := range section.Open {
		if !branch.Complete {
			t.Fatal("branch evidence")
		}
		for _, end := range branch.Ends {
			if end.Operand != 0 || (end.Side != geom.SurfaceEdgeVMin && end.Side != geom.SurfaceEdgeVMax) {
				t.Fatal("original terminal owner", end.Operand, end.Side)
			}
		}
	}
	t.Log("projected candidates", len(report.Projected), "generic fallbacks", len(report.GenericFallback), "operations", report.ExactOperations)
	missing := intersect.SplineSeedReport{}
	for _, seed := range section.Trace.Seeds.Attempts {
		if seed.Cell.DomainA.UMax < .5 {
			missing.Attempts = append(missing.Attempts, seed)
		}
	}
	if len(missing.Attempts) == 0 || len(missing.Attempts) == len(section.Trace.Seeds.Attempts) {
		t.Fatal("bad omission fixture")
	}
	bad, e := SectionSplinePairAlgebraic(ctx, pa, pb, &missing, o, geom.TubeAlgebraicGraphEndpointOptions{})
	if e == nil || bad.Complete || bad.Trace.Coverage.CoversAllContacts {
		t.Fatal("omitted branch accepted", e)
	}
	// Reuse only source proposals; every exact proof is re-run under the cap.
	o.Coverage.CommonXYProjected = &intersect.CommonXYProjectedCoverageOptions{MaxOperations: report.ExactOperations}
	exact, e := intersect.CertifySplineTubeCoverage(ctx, pa, pb, section.Trace.Seeds, o.Coverage)
	if e != nil || !exact.CoversAllContacts {
		t.Fatal("exact cumulative quota", e)
	}
	o.Coverage.CommonXYProjected = &intersect.CommonXYProjectedCoverageOptions{MaxOperations: report.ExactOperations - 1}
	limited, e := intersect.CertifySplineTubeCoverage(ctx, pa, pb, section.Trace.Seeds, o.Coverage)
	if e == nil || limited.CoversAllContacts || len(limited.Unresolved)+len(limited.Unchecked) == 0 {
		t.Fatal("cumulative deficit", e)
	}
}
