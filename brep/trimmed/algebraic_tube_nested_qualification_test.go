package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestNestedQuarticTubeMaterial(t *testing.T) {
	if testing.Short() {
		t.Skip("independent coverage and exact clipping of nested quartic tubes")
	}
	ctx := context.Background()
	r := nestedTubeMaterialFixture(t)
	annulus := -1
	for i, x := range r.Regions() {
		top := r.Topology(i)
		if len(x.Holes) == 1 {
			annulus = i
			if top.EulerCharacteristic != 0 || top.InnerWires != 2 || x.UpperChart != 0 ||
				r.Loops()[x.Holes[0]].Parent != i {
				t.Fatal(top, x, r.Loops())
			}
		} else if len(x.Holes) != 0 || top.EulerCharacteristic != 2 || x.UpperChart != 1 || r.Loops()[i].Parent < 0 {
			t.Fatal(top, x, r.Loops())
		}
	}
	if annulus < 0 {
		t.Fatal("missing annular component")
	}
	s, e := r.Solid(ctx, annulus, 1e-7, edgecheck.Options{})
	if e != nil || s.Topology().EulerCharacteristic != 0 || len(s.Faces()) != 2 {
		t.Fatalf("native annulus topology %+v err=%v", s.Topology(), e)
	}
	volume, e := r.BoundVolumes(ctx, AlgebraicTubeLensVolumeOptions{MaxDepth: 8, Patch: geom.SurfacePatchOptions{MaxOperations: 256000000}})
	if e != nil {
		t.Fatal(volume, e)
	}
	lo, _ := volume.Regions[annulus].Lower.Float64()
	hi, _ := volume.Regions[annulus].Upper.Float64()
	want := math.Pi * 9 / 24 * math.Pow(25.0/256-25.0/1024, 3)
	if lo <= 0 || lo > want || hi < want {
		t.Fatalf("annulus volume [%g,%g] excludes %g", lo, hi, want)
	}
	checkNestedTubeFootprint(t, r, annulus)
	checkNestedTubeRegionShell(t, r, annulus)
	base, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Z: -1}, {Y: 1, Z: -1}, {X: 1, Z: -1}, {X: 1, Y: 1, Z: -1}},
		[]float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	env, h, e := r.section.AssembleAlgebraicTubeGraphEnvelope(ctx, base, parametric.Union, 1e-7,
		AlgebraicTubeGraphBooleanOptions{Regions: nestedTubeRegionOptions()}, edgecheck.Options{})
	if e != nil || env.Topology().EulerCharacteristic != 2 || len(env.Faces()) != 8 ||
		len(h.TopFaces[0]) != 1 || len(h.TopFaces[1]) != 2 {
		t.Fatalf("nested graph envelope topology %+v history %+v err=%v", env.Topology(), h, e)
	}
	var native bytes.Buffer
	if e := WriteAlgebraicTubeRegionsJSON(&native, r); e != nil {
		t.Fatal(e)
	}
	back, e := ReadAlgebraicTubeRegionsJSON(ctx, bytes.NewReader(native.Bytes()), JSONLimits{}, nestedTubeSectionOptions(), nestedTubeRegionOptions())
	if e != nil || len(back.Regions()) != 2 {
		t.Fatalf("reloaded region count %d err=%v", len(back.Regions()), e)
	}
	for _, q := range []struct {
		u, v    *big.Rat
		present bool
		upper   int
	}{
		{big.NewRat(33, 64), big.NewRat(33, 64), true, 1},
		{big.NewRat(47, 64), big.NewRat(33, 64), true, 0},
		{big.NewRat(61, 64), big.NewRat(33, 64), false, -1},
	} {
		x, e := r.VerticalInterval(ctx, [2]*big.Rat{q.u, q.v}, geom.ExactCurvePlaneEventOptions{MaxOperations: 64000000})
		if e != nil || !x.Complete || x.Present != q.present || q.present && r.Regions()[x.Region].UpperChart != q.upper {
			t.Fatalf("query %+v result=%+v err=%v", q, x, e)
		}
	}
}

func nestedTubeMaterialFixture(t *testing.T) AlgebraicTubeRegions {
	t.Helper()
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	x2 := [5]float64{3, 0, -1, 0, 3}
	x4 := [5]float64{1, -1, 1, -1, 1}
	const ri, ro = 25.0 / 1024, 25.0 / 256
	var net []geom.Vec3
	for i := range x2 {
		for j := range x2 {
			z := (9*(x4[i]+x4[j]) + 2*x2[i]*x2[j] - 12*(ri+ro)*(x2[i]+x2[j]) + 144*ri*ro) / 64
			net = append(net, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: z})
		}
	}
	k = []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	b, e := geom.NewBSplineSurface(4, 4, 5, 5, net, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	v := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	var loop []Use
	for i, p := range v {
		loop = append(loop, Use{Edge: i, UVStart: p, UVEnd: v[(i+1)%4]})
	}
	p := [2]intersect.Patch{{Surface: a, Max: v[2]}, {Surface: b, Max: v[2]}}
	f := [2]Face{{Surface: a, Outer: loop}, {Surface: b, Outer: loop}}
	o := nestedTubeSectionOptions()
	h, e := intersect.DiscoverRadialGraphPairTubes(ctx, p[0], p[1], geom.SurfacePatchOptions{}, intersect.SplinePlanePairDiscoveryOptions{MaxTubes: 128})
	if e != nil {
		t.Fatal(e)
	}
	s, e := SectionTrimmedSplinePair(ctx, p, f, &h, o)
	if e != nil || !s.Complete || len(s.Branches) != 2 {
		t.Fatalf("section complete=%v branches=%d issues=%v err=%v", s.Complete, len(s.Branches), s.Trace.Issues, e)
	}
	r, e := s.SealAlgebraicTubeRegions(ctx, nestedTubeRegionOptions())
	if e != nil || len(r.Regions()) != 2 {
		t.Fatalf("regions=%+v err=%v", r.Regions(), e)
	}
	return r
}

func nestedTubeSectionOptions() AlgebraicTrimmedPairSectionOptions {
	o := AlgebraicTrimmedPairSectionOptions{MaxSegments: 256}
	o.Clip.MaxOperations = 512000000
	o.Clip.Loops.MaxWork = 16000000
	o.Trace.Branches.PruneSeparatedEndpoints = true
	o.Trace.Branches.MaxComparisons = 4096
	o.Trace.MaxTopologyPairs = 1000000
	o.Trace.Coverage.MaxCandidateTests = 4096
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	return o
}

func nestedTubeRegionOptions() AlgebraicTubeRegionOptions {
	o2 := AlgebraicTubeRegionOptions{}
	o2.Exact.MaxOperations = 512000000
	o2.Partition.MaxOperations = 256000000
	return o2
}
