package trimmed

import (
	"context"
	"errors"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestSimultaneousAlgebraicSplineOwnerLedger(t *testing.T) {
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

	for order, pair := range [][2]intersect.Patch{{pa, pb}, {pb, pa}} {
		result, err := SectionSplinePairAlgebraicOwners(ctx, pair[0], pair[1], nil, o, geom.TubeAlgebraicGraphEndpointOptions{})
		if err != nil || !result.Complete || len(result.Open) != 2 || result.Closed != 0 {
			t.Fatal("paired ledger", len(result.Open), err)
		}
		for _, branch := range result.Open {
			if branch.Segments < 1 || len(branch.Junctions) != branch.Segments-1 {
				t.Fatal("junction ownership")
			}
			for _, end := range branch.Ends {
				owners := end.Owners()
				if owners[0].Operand != 0 || owners[1].Operand != 1 || owners[0].Side != owners[1].Side {
					t.Fatal("original occurrences", owners)
				}
				equal, _, err := owners[0].ModelEvent.CompareModelPoint(ctx, owners[1].ModelEvent, geom.ExactCurvePlaneEventOptions{})
				if err != nil || !equal {
					t.Fatal("model equality", err)
				}
				for _, owner := range owners {
					if owner.Parameter == nil || owner.EdgeParameter == nil || owner.ModelEvent == nil {
						t.Fatal("missing root lineage")
					}
				}
				owners[0].Operand = 99
				if end.Owners()[0].Operand != 0 {
					t.Fatal("mutable owner slice")
				}
			}
		}
		first := result.Trace.Curves[0][0]
		terminal, err := DiscoverAlgebraicSplineTerminalPair(ctx, first, 0, pair[0], pair[1], geom.TubeAlgebraicGraphEndpointOptions{})
		if err != nil || terminal == nil {
			t.Fatal("standalone pair", err)
		}
		if _, err = DiscoverAlgebraicSplineTerminalPair(ctx, first, 0, pair[1], pair[0], geom.TubeAlgebraicGraphEndpointOptions{}); err == nil {
			t.Fatal("unbound original charts")
		}
		if _, err = DiscoverAlgebraicSplineTerminalPair(ctx, result.Trace.Curves[0][1], 0, pair[0], pair[1], geom.TubeAlgebraicGraphEndpointOptions{}); err == nil {
			t.Fatal("interior junction promoted to boundary")
		}
		wrong, err := discoverExactTubeSectionEndpoint(ctx, first, 0, pair[0], pair[1], geom.TubeAlgebraicGraphEndpointOptions{})
		if err != nil {
			t.Fatal(err)
		}
		wrong.ModelEvent = result.Open[1].Ends[0].ModelEvent()
		budget, _ := newPairedTerminalBudget(geom.TubeAlgebraicGraphEndpointOptions{})
		if rejected, err := certifyPairedTerminal(ctx, wrong, pair[0], pair[1], budget); err == nil || rejected != nil {
			t.Fatal("mismatched exact model event accepted")
		}

		missing := intersect.SplineSeedReport{}
		for _, seed := range result.Trace.Seeds.Attempts {
			if seed.Cell.DomainA.UMax < .5 {
				missing.Attempts = append(missing.Attempts, seed)
			}
		}
		bad, err := SectionSplinePairAlgebraicOwners(ctx, pair[0], pair[1], &missing, o, geom.TubeAlgebraicGraphEndpointOptions{})
		if err == nil || bad.Complete || len(bad.Open) != 0 || len(bad.Trace.Curves) != 0 {
			t.Fatal("missing branch published ledger", err)
		}
		if order == 0 {
			cap := geom.TubeAlgebraicGraphEndpointOptions{}
			cap.MaxOperations = result.ExactOperations - 1
			truncated, err := SectionSplinePairAlgebraicOwners(ctx, pair[0], pair[1], &result.Trace.Seeds, o, cap)
			if err == nil || truncated.Complete || len(truncated.Open) != 0 {
				t.Fatal("cumulative ledger budget not enforced", result.ExactOperations, err)
			}
		}

		limited := geom.TubeAlgebraicGraphEndpointOptions{}
		limited.MaxOperations = 1
		bad, err = SectionSplinePairAlgebraicOwners(ctx, pair[0], pair[1], &result.Trace.Seeds, o, limited)
		if !errors.Is(err, geom.ErrConvergence) || bad.Complete || len(bad.Open) != 0 {
			t.Fatal("atomic work limit", err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	bad, err := SectionSplinePairAlgebraicOwners(canceled, pa, pb, nil, o, geom.TubeAlgebraicGraphEndpointOptions{})
	if !errors.Is(err, context.Canceled) || bad.Complete {
		t.Fatal("cancellation", err)
	}
}
