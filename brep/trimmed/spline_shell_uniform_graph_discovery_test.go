package trimmed

import (
	"context"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/intersect"
	"testing"
)

func TestAutomaticMultipleGraphShellPairLedger(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1.0 / 64}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	var poles []geom.Vec3
	for i, z := range []float64{3.0 / 16, -5.0 / 16, 3.0 / 16} {
		for j, dz := range []float64{0, 1.0 / 128, 0} {
			poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z + dz + float64(i*j)/256})
		}
	}
	b, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := intersect.Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, intersect.Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}

	o := intersect.SplinePairTraceOptions{}
	o.Coverage.Isolation.ParameterTolerance = .125
	o.Coverage.Cell.MaxOperations = 16000000
	o.Branches.PruneSeparatedEndpoints = true
	o.Branches.MaxComparisons = 4096
	o.MaxTopologyPairs = 1000000
	o.Coverage.MaxCandidateTests = 4096
	d := intersect.UniformGraphDiscoveryOptions{GridDivisions: 16, AxisDivisions: [2]int{8, 16}}
	d.Plane.Arcs.ResolveSubdivisionCorners = true
	d.MaxArcs = 1024
	d.MaxTubes = 1024

	o.UniformGraphFallback = d
	o.UniformGraphFallback.Enabled = true

	section, e := SectionSplinePairAlgebraic(ctx, pa, pb, nil, o, geom.TubeAlgebraicGraphEndpointOptions{})
	// Both charts meet their original V boundary simultaneously. The current
	// algebraic shell terminal requires one unique boundary owner; do not choose
	// one arbitrarily or publish a completed native shell section.
	if !isUnresolvedOpenSpline(e) || section.Complete || !section.Trace.Complete || len(section.Trace.Curves) != 2 {
		t.Fatal("simultaneous owner boundary", section.Complete, section.Trace.Issues, e)
	}
	for _, curves := range section.Trace.Curves {
		for end, seg := range []intersect.TubeSegment{curves[0], curves[len(curves)-1]} {
			proof, err := discoverExactTubeSectionEndpoint(ctx, seg, end, pa, pb, geom.TubeAlgebraicGraphEndpointOptions{})
			if err != nil || !proof.Proven {
				t.Fatal("original endpoint proof", err)
			}
			if _, _, err = locateAlgebraicTerminalBoundary(proof, pa, pb); !isUnresolvedOpenSpline(err) {
				t.Fatal("two boundary owners must not collapse", err)
			}
		}
	}
	proposals, e := proposeGeneralSplineSections(ctx, pa, pb, [2]int{3, 7}, SplineShellOpenOptions{Trace: o})
	if e != nil || len(proposals) != 2 {
		t.Fatal("native owner proposals", len(proposals), e)
	}
	for i, p := range proposals {
		if p.FaceA != 3 || p.FaceB != 7 || p.Branch != i || p.Closed || p.Start.OtherSide == nil || p.End.OtherSide == nil {
			t.Fatal("owner identity", p)
		}
	}
	if len(proposals[0].TubeCells.Attempts) == 0 || len(proposals[1].TubeCells.Attempts) != 0 {
		t.Fatal("pair-wide seed ownership")
	}
	missing := intersect.SplineSeedReport{}
	for _, seed := range section.Trace.Seeds.Attempts {
		if seed.Cell.DomainA.UMax < .5 {
			missing.Attempts = append(missing.Attempts, seed)
		}
	}
	bad, e := SectionSplinePairAlgebraic(ctx, pa, pb, &missing, o, geom.TubeAlgebraicGraphEndpointOptions{})
	if e == nil || bad.Complete {
		t.Fatal("missing branch bypassed ledger coverage")
	}
}
