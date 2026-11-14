package intersect_test

import (
	"context"
	"math/big"
	"os"
	"plainsolid/geom"
	"plainsolid/internal/testgraphfixture"
	"plainsolid/intersect"
	"testing"
)

func TestTripleRadialPhysicalTransferCoverage(t *testing.T) {
	if os.Getenv("PLAINSOLID_RADIAL_TRANSFER_TRIPLE") != "1" {
		t.Skip("opt-in original triple pair coverage; no mesh")
	}
	ctx := context.Background()
	a, b, e := testgraphfixture.TripleRadialGraphs()
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := intersect.Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, intersect.Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}
	center := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	radii := testgraphfixture.TripleRadialRadii()
	proposals := intersect.SplinePlanePairDiscoveryOptions{}
	proposals.Tubes.MaxOperations = 512000000
	seeds, e := intersect.DiscoverRadialGraphContourTubes(ctx, pa, pb, center, radii, geom.SurfacePatchOptions{}, proposals)
	if e != nil {
		t.Fatal(e)
	}
	t.Log("original tubes", len(seeds.Attempts))
	o := intersect.SplineTubeCoverageOptions{RadialOverlap: &intersect.RadialOverlapCoverageOptions{AffineCharts: true, Center: center, Radii: radii, Proof: geom.RadialTubeCoverageOptions{MaxOperations: 512000000, MaxTubes: 512}}, Cell: geom.SurfaceTubeUnionCoverageOptions{MaxOperations: 16000000}}
	report, e := intersect.CertifySplineTubeCoverage(ctx, pa, pb, seeds, o)
	if e != nil || !report.CoversAllContacts || report.RadialOverlap == nil {
		t.Fatal("complete original pair coverage", e)
	}
	proof := report.RadialOverlap
	if proof.TransferMode != intersect.RadialTransferPhysical || proof.TransferStrata == 0 || proof.TransferTests == 0 {
		t.Fatal(proof)
	}
	t.Log("transfer mode", proof.TransferMode, "overlap cells", proof.Transferred, "operations", proof.ExactOperations, "strata", proof.TransferStrata, "uncovered strata tests", proof.TransferTests)
}
