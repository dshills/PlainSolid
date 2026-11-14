package intersect

import (
	"context"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestVerifiedSingletonTubeJunctionReuse(t *testing.T) {
	ctx := context.Background()
	a, b, seeds := mixedTubeFixture(t)
	pa, e := exactSplinePatch(a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pb, e := exactSplinePatch(b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var tubes [2]geom.VerifiedSurfaceTube
	var ends [2]geom.SurfaceTubeEvaluation
	for i, seed := range seeds.Attempts {
		x, e := pa.Trim(seed.Cell.DomainA)
		if e != nil {
			t.Fatal(e)
		}
		y, e := pb.Trim(seed.Cell.DomainB)
		if e != nil {
			t.Fatal(e)
		}
		tubes[i], e = geom.NewVerifiedSurfaceTube(x, y, geom.SurfacePairTubeOptions{FreeAxis: seed.Tube.FreeAxis})
		if e != nil {
			t.Fatal(e)
		}
		ends[i], e = tubes[i].Evaluate(geom.SurfaceTubeEvaluationOptions{Parameter: .5})
		if e != nil {
			t.Fatal(e)
		}
	}
	old, e := geom.CertifySurfaceTubeEndpointJunction(pa, pb, seeds.Attempts[0].Cell.DomainA, seeds.Attempts[0].Cell.DomainB, seeds.Attempts[1].Cell.DomainA, seeds.Attempts[1].Cell.DomainB, geom.SurfaceTubeEndpointOptions{LeftFreeAxis: 0, RightFreeAxis: 2, LeftEnd: 1, RightEnd: 0})
	if e != nil || !old.Connected {
		t.Fatal(old, e)
	}
	connected, work, e := verifiedSingletonTubeJunction(ctx, tubes[0], tubes[1], 1, 0, ends[0], ends[1], geom.SurfaceTubeJunctionOptions{})
	if e != nil || !connected || work <= 0 || work >= old.ExactOperations {
		t.Fatal(connected, work, old.ExactOperations, e)
	}
	t.Logf("junction exact operations: old=%d, retained-proof path=%d", old.ExactOperations, work)
	for _, limit := range []int{work, work - 1} {
		yes, _, e := verifiedSingletonTubeJunction(ctx, tubes[0], tubes[1], 1, 0, ends[0], ends[1], geom.SurfaceTubeJunctionOptions{MaxOperations: limit})
		if limit == work {
			if e != nil || !yes {
				t.Fatal(e)
			}
		} else if e == nil || yes {
			t.Fatal("exhausted endpoint proof accepted")
		}
	}
	// Fabricated singleton enclosures are proposals only, not source evidence.
	forged := ends
	for i := range forged {
		forged[i].ParametersA.Min.Y = math.Nextafter(.5, 1)
		forged[i].ParametersA.Max.Y = forged[i].ParametersA.Min.Y
	}
	if yes, _, _ := verifiedSingletonTubeJunction(ctx, tubes[0], tubes[1], 1, 0, forged[0], forged[1], geom.SurfaceTubeJunctionOptions{}); yes {
		t.Fatal("one-ULP false hint accepted")
	}
	wide := ends[0]
	wide.Bounds.Min.Y = math.Nextafter(.5, 0)
	if yes, work, e := verifiedSingletonTubeJunction(ctx, tubes[0], tubes[1], 1, 0, wide, ends[1], geom.SurfaceTubeJunctionOptions{}); e != nil || yes || work != 0 {
		t.Fatal("nonsingleton did work", yes, work, e)
	}
	graph, e := JoinSplineTubeBranches(ctx, a, b, seeds, TubeBranchOptions{})
	if e != nil || len(graph.Branches) != 1 || graph.RationalEndpointOperations <= 0 {
		t.Fatal(graph, e)
	}
	curves, e := SplineBranchCurves(ctx, a, b, seeds, graph.Branches[0], TubeCurveOptions{})
	if e != nil || len(curves) != 2 {
		t.Fatal(curves, e)
	}
	// Same chart parameters on an independently translated support pair cannot sew.
	k := []float64{0, 0, 1, 1}
	sa, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Z: .125}, {Y: 1, Z: .125}, {X: 1, Z: .125}, {X: 1, Y: 1, Z: .125}}, k, k, nil)
	sb, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Y: .5, Z: 1.125}, {Y: .5, Z: -.875}, {X: 1, Y: .5, Z: 1.125}, {X: 1, Y: .5, Z: -.875}}, k, k, nil)
	qa, _ := geom.NewSurfacePatch(sa, geom.SurfacePatchOptions{})
	qb, _ := geom.NewSurfacePatch(sb, geom.SurfacePatchOptions{})
	qa, _ = qa.Trim(seeds.Attempts[1].Cell.DomainA)
	qb, _ = qb.Trim(seeds.Attempts[1].Cell.DomainB)
	other, e := geom.NewVerifiedSurfaceTube(qa, qb, geom.SurfacePairTubeOptions{FreeAxis: 2})
	if e != nil {
		t.Fatal(e)
	}
	if yes, _, e := verifiedSingletonTubeJunction(ctx, tubes[0], other, 1, 0, ends[0], ends[1], geom.SurfaceTubeJunctionOptions{}); e != nil || yes {
		t.Fatal("source-mismatched model endpoint", yes, e)
	}
}
