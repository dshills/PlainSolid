package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestNonlinearTrapezoidWallSection(t *testing.T) {
	ctx := context.Background()
	a, _ := crossingFootprintRoofShellShape(t, false, false)
	b, _ := crossingFootprintRoofShellShape(t, true, false)
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
	o.Trace.Coverage.IsoPlane.AllowTrapezoidCharts = true
	o.Trace.PlaneFallback.Enabled = true
	o.Trace.PlaneFallback.TrapezoidCharts = true
	o.Trace.Coverage.IsoPlane.MaxOperations = 64000000
	o.Trace.PlaneFallback.Plane.Arcs.ResolveSubdivisionCorners = true

	pa, e := exactFaceSupportPatch(a.faces[5], geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pb, e := exactFaceSupportPatch(b.faces[1], geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	aa, _ := algebraicShellTracePatch(a.faces[5], pa.Domain())
	bb, _ := algebraicShellTracePatch(b.faces[1], pb.Domain())

	chart, err := geom.NewTrapezoidSurfaceChart(ctx, pa, geom.AffineSurfaceChartOptions{})
	if err != nil {
		t.Fatal(err)
	}
	iso, err := pb.IsoCurve(1, 1, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	clipped, err := geom.ClipIsoCurveToTrapezoidChart(ctx, iso, chart, geom.SplinePlaneOptions{})
	if err != nil || !clipped.Complete || len(clipped.Spans) != 1 {
		t.Fatal("original interval", err)
	}
	if clipped.Spans[0].Start.Parameter.Cmp(big.NewRat(1, 4)) != 0 || clipped.Spans[0].End.Parameter.Cmp(big.NewRat(3, 4)) != 0 {
		t.Fatal("wrong complete domain")
	}
	for order, pair := range [][2]intersect.Patch{{aa, bb}, {bb, aa}} {
		proposal, err := intersect.DiscoverSplinePlanePairSections(ctx, pair[0], pair[1], geom.SurfacePatchOptions{}, o.Trace.PlaneFallback)
		if err != nil || proposal.OpenContours != 1 || len(proposal.Seeds.Attempts) != 4 {
			t.Fatal("original proposals", order, err)
		}
		anchors := 0
		for _, seed := range proposal.Seeds.Attempts {
			if seed.BoundaryAnchor != nil {
				anchors++
			}
		}
		if anchors != 2 {
			t.Fatal("missing exact terminal anchors", anchors)
		}
		section, err := SectionSplinePairAlgebraic(ctx, pair[0], pair[1], nil, o.Trace, geom.TubeAlgebraicGraphEndpointOptions{})
		if err != nil || !section.Complete || len(section.Open) != 1 || !section.Trace.Coverage.CoversAllContacts {
			t.Fatal("complete original pair", order, section.Trace.Issues, err)
		}
		branch := section.Open[0]
		if !branch.Complete || branch.Segments != 4 || len(branch.Junctions) != 3 {
			t.Fatal("missing original junction")
		}
		for _, end := range branch.Ends {
			if end.Operand != order || end.Side != geom.SurfaceEdgeVMax || !end.Certificate.Proven {
				t.Fatal("lost original side", end)
			}
		}
		for _, omit := range []int{0, 1, 3} {
			missing := proposal.Seeds
			missing.Attempts = append([]intersect.SplineSeedCell(nil), proposal.Seeds.Attempts[:omit]...)
			missing.Attempts = append(missing.Attempts, proposal.Seeds.Attempts[omit+1:]...)
			bad, err := intersect.CertifyIsoPlaneSplineTubeCoverage(ctx, pair[0], pair[1], missing, o.Trace.Coverage.IsoPlane)
			if err == nil || bad.Covered || len(bad.AcceptedTubes) != 0 {
				t.Fatal("missing interval accepted", order, omit, err)
			}
		}
		falseAnchor := proposal.Seeds
		falseAnchor.Attempts = append([]intersect.SplineSeedCell(nil), proposal.Seeds.Attempts...)
		for i := range falseAnchor.Attempts {
			if falseAnchor.Attempts[i].BoundaryAnchor != nil {
				copy := *falseAnchor.Attempts[i].BoundaryAnchor
				copy.Parameters[1+2*order] = math.Nextafter(copy.Parameters[1+2*order], 0)
				falseAnchor.Attempts[i].BoundaryAnchor = &copy
				break
			}
		}
		bad, err := intersect.CertifyIsoPlaneSplineTubeCoverage(ctx, pair[0], pair[1], falseAnchor, o.Trace.Coverage.IsoPlane)
		if err == nil || bad.Covered || len(bad.AcceptedTubes) != 0 {
			t.Fatal("false wall anchor accepted", err)
		}
		limited := o.Trace.Coverage.IsoPlane
		limited.MaxOperations = 1
		bad, err = intersect.CertifyIsoPlaneSplineTubeCoverage(ctx, pair[0], pair[1], proposal.Seeds, limited)
		if err == nil || bad.Covered || len(bad.Spans) != 0 {
			t.Fatal("atomic work limit", err)
		}
		t.Logf("order=%d tubes=%d anchors=%d complete interval=%s..%s", order, len(section.Trace.Seeds.Attempts), anchors, clipped.Spans[0].Start.Parameter, clipped.Spans[0].End.Parameter)
	}
	lastPatch, err := exactFaceSupportPatch(b.faces[3], geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	lastIso, err := lastPatch.IsoCurve(0, 1, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	lastClip, err := geom.ClipIsoCurveToTrapezoidChart(ctx, lastIso, chart, geom.SplinePlaneOptions{})
	if err != nil || !lastClip.Complete || len(lastClip.Spans) != 1 {
		t.Fatal("last exact interval", err)
	}
	lastSpan := lastClip.Spans[0]
	if lastSpan.Start.Parameter.Sign() != 0 || lastSpan.Start.UV[1].Cmp(big.NewRat(32, 65)) != 0 || lastSpan.End.Parameter.Cmp(big.NewRat(11, 15)) != 0 || lastSpan.End.UV[1].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("nonbinary terminal provenance", lastSpan)
	}
	// This advances the original shell, but the last wall/wall pair still needs
	// different free charts at its two nonbinary endpoints. No wire is published.
	full, err := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if !errors.Is(err, intersect.ErrUnresolved) || full.Complete || len(full.Wires) != 0 || len(full.Branches) != 11 || full.Pairs[len(full.Pairs)-1] != [2]int{5, 3} {
		t.Fatal("unsupported global frontier", full.Complete, full.Pairs, err)
	}
}
