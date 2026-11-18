package intersect_test

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/internal/testgraphfixture"
	"plainsolid/intersect"
	"testing"
)

func TestTripleRadialOriginalTubeDiscovery(t *testing.T) {
	slowtest.Skip(t)
	if testing.Short() {
		t.Skip("complete three-contour original coverage and topology")
	}
	ctx := context.Background()
	a, b, e := testgraphfixture.TripleRadialGraphs()
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := intersect.Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, intersect.Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}
	center := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	o := intersect.SplinePlanePairDiscoveryOptions{}
	o.Tubes.MaxOperations = 512000000
	seeds, e := intersect.DiscoverRadialGraphContourTubes(ctx, pa, pb, center, testgraphfixture.TripleRadialRadii(), geom.SurfacePatchOptions{}, o)
	if e != nil {
		t.Fatal(e)
	}
	t.Log("proved original tubes", len(seeds.Attempts))
	trace := intersect.SplinePairTraceOptions{}
	trace.Branches.PruneSeparatedEndpoints = true
	trace.Branches.MaxComparisons = 4096
	trace.MaxTopologyPairs = 1000000
	trace.Coverage.MaxCandidateTests = 4096
	trace.Coverage.Isolation.ParameterTolerance = .125
	trace.Coverage.Cell.MaxOperations = 16000000
	result, e := intersect.CertifySplinePairBranches(ctx, pa, pb, seeds, trace)
	if e != nil || !result.Complete || len(result.Curves) != 3 {
		t.Fatal("three complete branches", len(result.Curves), result.Issues, e)
	}
	for _, branch := range result.BranchGraph.Branches {
		if !branch.Closed || branch.StartUnresolved || branch.EndUnresolved {
			t.Fatal("not a complete closed original branch", branch)
		}
	}
}

func TestRadialContourDiscoveryAtomicRejections(t *testing.T) {
	a, b, e := testgraphfixture.TripleRadialGraphs()
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := intersect.Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, intersect.Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}
	center := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	for _, tc := range []struct {
		name    string
		radii   []*big.Rat
		options intersect.SplinePlanePairDiscoveryOptions
	}{
		{name: "omitted contour", radii: testgraphfixture.TripleRadialRadii()[1:]},
		{name: "tube quota", radii: testgraphfixture.TripleRadialRadii(), options: intersect.SplinePlanePairDiscoveryOptions{MaxTubes: 1}},
		{name: "arithmetic quota", radii: testgraphfixture.TripleRadialRadii(), options: intersect.SplinePlanePairDiscoveryOptions{Tubes: geom.SurfaceArcTubeDiscoveryOptions{MaxOperations: 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, e := intersect.DiscoverRadialGraphContourTubes(context.Background(), pa, pb, center, tc.radii, geom.SurfacePatchOptions{}, tc.options)
			if e == nil || len(r.Attempts) != 0 {
				t.Fatalf("partial or accepted report: %d %v", len(r.Attempts), e)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := intersect.DiscoverRadialGraphContourTubes(ctx, pa, pb, center, testgraphfixture.TripleRadialRadii(), geom.SurfacePatchOptions{}, intersect.SplinePlanePairDiscoveryOptions{})
	if e != context.Canceled || len(r.Attempts) != 0 {
		t.Fatal("cancellation", e)
	}
}

func TestTripleRadialCoverageRejectsOmittedInnerBranch(t *testing.T) {
	slowtest.Skip(t)
	if testing.Short() {
		t.Skip("fresh whole-domain omitted-contour coverage proof")
	}
	ctx := context.Background()
	a, b, e := testgraphfixture.TripleRadialGraphs()
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := intersect.Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, intersect.Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}
	seeds, e := intersect.DiscoverRadialGraphContourTubes(ctx, pa, pb, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, testgraphfixture.TripleRadialRadii(), geom.SurfacePatchOptions{}, intersect.SplinePlanePairDiscoveryOptions{})
	if e != nil {
		t.Fatal(e)
	}
	missing := intersect.SplineSeedReport{}
	for _, seed := range seeds.Attempts {
		d := seed.Cell.DomainB
		// The small circle lies wholly inside this rational square, the other two
		// wholly outside. This selects omitted proposals only, not a zero-set proof.
		if d.UMin > 3.0/8 && d.UMax < 5.0/8 && d.VMin > 3.0/8 && d.VMax < 5.0/8 {
			continue
		}
		missing.Attempts = append(missing.Attempts, seed)
	}
	if len(missing.Attempts) == 0 || len(missing.Attempts) == len(seeds.Attempts) {
		t.Fatal("test did not omit precisely an inner family")
	}
	options := intersect.SplineTubeCoverageOptions{MaxCandidateTests: 4096}
	options.Isolation.ParameterTolerance = .125
	options.Cell.MaxOperations = 16000000
	report, e := intersect.CertifySplineTubeCoverage(ctx, pa, pb, missing, options)
	if e == nil || report.CoversAllContacts {
		t.Fatal("fresh coverage accepted a missing physical contour", e)
	}
	if len(report.Unresolved) == 0 {
		t.Fatal("omission must be an uncovered contact, not an invalid input or quota", e)
	}
	t.Log("omitted tubes", len(seeds.Attempts)-len(missing.Attempts), "uncovered original cells", len(report.Unresolved))
}
