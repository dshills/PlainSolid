package intersect

import (
	"context"
	"errors"

	"plainsolid/geom"
	"testing"
)

// Two curved graph supports meet on two disjoint curved open branches.
// Intermediate dependent UV coordinates are generally irrational.
func TestUniformGraphMultipleContours(t *testing.T) {
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
	pa, pb := Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}

	o := SplinePairTraceOptions{}
	o.Coverage.Isolation.ParameterTolerance = .125
	o.Coverage.Cell.MaxOperations = 16000000
	o.Branches.PruneSeparatedEndpoints = true
	o.Branches.MaxComparisons = 4096
	o.MaxTopologyPairs = 1000000
	o.Coverage.MaxCandidateTests = 4096
	d := UniformGraphDiscoveryOptions{GridDivisions: 16, AxisDivisions: [2]int{8, 16}}
	d.Plane.Arcs.ResolveSubdivisionCorners = true
	d.MaxArcs = 1024
	d.MaxTubes = 1024
	seeds, e := DiscoverUniformGraphPairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, d)
	if e != nil {
		t.Fatal("discover", len(seeds.Attempts), e)
	}
	t.Log("seeds", len(seeds.Attempts))
	r, e := CertifySplinePairBranches(ctx, pa, pb, seeds, o)
	if e != nil || !r.Complete || len(r.Curves) != 2 {
		t.Fatal("certify", r.Issues, len(r.Curves), e)
	}

	for _, branch := range r.BranchGraph.Branches {
		if branch.Closed {
			t.Fatal("open original branch classified closed")
		}
	}
	reversed, err := DiscoverUniformGraphPairTubes(ctx, pb, pa, geom.SurfacePatchOptions{}, d)
	if err != nil {
		t.Fatal(err)
	}
	reverseProof, err := CertifySplinePairBranches(ctx, pb, pa, reversed, o)
	if err != nil || !reverseProof.Complete || len(reverseProof.Curves) != 2 {
		t.Fatal("reversed sources", reverseProof.Issues, err)
	}

	without, withoutErr := TraceSplinePair(ctx, pa, pb, o)
	if withoutErr == nil || without.Complete {
		t.Fatal("fixture no longer exercises fallback")
	}
	o.UniformGraphFallback = d
	o.UniformGraphFallback.Enabled = true
	auto, err := TraceSplinePair(ctx, pa, pb, o)
	if err != nil || !auto.Complete || len(auto.Curves) != 2 {
		t.Fatal("automatic fallback", auto.Issues, err)
	}

	// Remove every tube of the right component, not merely one local segment.
	missing := SplineSeedReport{}
	for _, seed := range seeds.Attempts {
		if seed.Cell.DomainA.UMax < .5 {
			missing.Attempts = append(missing.Attempts, seed)
		}
	}
	if len(missing.Attempts) == 0 || len(missing.Attempts) == len(seeds.Attempts) {
		t.Fatal("bad omission fixture")
	}
	bad, err := CertifySplinePairBranches(ctx, pa, pb, missing, o)
	if err == nil || bad.Complete {
		t.Fatal("omitted component accepted", bad.Issues, err)
	}
	limited := d
	limited.MaxTubes = 1
	partial, err := DiscoverUniformGraphPairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, limited)
	if err == nil || len(partial.Attempts) != 0 {
		t.Fatal("partial publication", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = DiscoverUniformGraphPairTubes(canceled, pa, pb, geom.SurfacePatchOptions{}, d); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	invalid := d
	invalid.AxisDivisions[0] = 3
	if _, err = DiscoverUniformGraphPairTubes(ctx, pa, pb, geom.SurfacePatchOptions{}, invalid); !errors.Is(err, geom.ErrInvalid) {
		t.Fatal(err)
	}
}
