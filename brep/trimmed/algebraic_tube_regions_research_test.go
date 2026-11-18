//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestSealAlgebraicTubeRegionsDisjoint(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	var net []geom.Vec3
	for i := range 5 {
		for j, z := range [3]float64{.25, -.25, .25} {
			u := 1.0 / 16
			if i%2 == 1 {
				u = -u
			}
			net = append(net, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 2, Z: u + z - 25.0/1024})
		}
	}
	b, e := geom.NewBSplineSurface(2, 2, 5, 3, net, []float64{0, 0, 0, .5, .5, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	v := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	loop := make([]Use, 4)
	for i, p := range v {
		loop[i] = Use{Edge: i, UVStart: p, UVEnd: v[(i+1)%4]}
	}
	p := [2]intersect.Patch{{Surface: a, Max: v[2]}, {Surface: b, Max: v[2]}}
	f := [2]Face{{Surface: a, Outer: loop}, {Surface: b, Outer: loop}}
	o := AlgebraicTrimmedPairSectionOptions{}
	o.Trace.GraphFallback.UCuts = []float64{.125, .15625, .34375, .375, .625, .65625, .84375, .875}
	o.Trace.GraphFallback.VCuts = []float64{.375, .40625, .59375, .625}
	o.Trace.GraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 32000000
	h, e := intersect.DiscoverCommonXYGraphPairTubes(ctx, p[0], p[1], geom.SurfacePatchOptions{}, o.Trace.GraphFallback)
	if e != nil {
		t.Fatal(len(h.Attempts), e)
	}
	s, e := SectionTrimmedSplinePair(ctx, p, f, &h, o)
	if e != nil || !s.Complete {
		t.Fatalf("section complete=%v branches=%d issues=%v err=%v", s.Complete, len(s.Branches), s.Trace.Issues, e)
	}
	r, e := s.SealAlgebraicTubeRegions(ctx, AlgebraicTubeRegionOptions{})
	if e != nil || len(r.Regions()) != 2 {
		t.Fatalf("regions=%+v err=%v", r.Regions(), e)
	}
	for i, x := range r.Regions() {
		top := r.Topology(i)
		if top.EulerCharacteristic != 2 || top.Faces != 2 || top.InnerWires != 0 || x.UpperChart != 0 ||
			r.Loops()[x.Outer].Parent != -1 || len(r.Boundary(x.Outer)) < 4 {
			t.Fatal(top, x, r.Loops())
		}
		f := r.RegionFaces(i)
		if f[0].Reversed || !f[1].Reversed || (f[0].OuterReversed != f[0].Reversed) == (f[1].OuterReversed != f[1].Reversed) {
			t.Fatal("inconsistent outward incidence", f)
		}
	}
	query := func(l AlgebraicTubeRegions, u, v *big.Rat) AlgebraicTubeRegionVerticalInterval {
		t.Helper()
		q, e := l.VerticalInterval(ctx, [2]*big.Rat{u, v}, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !q.Complete {
			t.Fatalf("vertical interval %+v: %v", q, e)
		}
		return q
	}
	first := query(r, big.NewRat(1, 4), big.NewRat(33, 64))
	second := query(r, big.NewRat(3, 4), big.NewRat(33, 64))
	if !first.Present || !second.Present || first.Region == second.Region || first.Lower.Cmp(big.NewRat(-99, 4096)) != 0 ||
		second.Lower.Cmp(first.Lower) != 0 || first.Upper.Sign() != 0 || second.Upper.Sign() != 0 {
		t.Fatal(first, second)
	}
	out := query(r, big.NewRat(1, 2), big.NewRat(33, 64))
	if out.Present {
		t.Fatal(out)
	}
	rim := query(r, big.NewRat(13, 32), big.NewRat(1, 2))
	if !rim.Present || !rim.Rim || rim.RimLoop < 0 || rim.Region != -1 || rim.Upper.Sign() != 0 {
		t.Fatal(rim)
	}
	var file bytes.Buffer
	if e := WriteAlgebraicTubeRegionsJSON(&file, r); e != nil {
		t.Fatal(e)
	}
	reloaded, e := ReadAlgebraicTubeRegionsJSON(ctx, bytes.NewReader(file.Bytes()), JSONLimits{}, o, AlgebraicTubeRegionOptions{})
	if e != nil || len(reloaded.Regions()) != 2 {
		t.Fatal(len(reloaded.Regions()), e)
	}
	if q := query(reloaded, big.NewRat(1, 4), big.NewRat(33, 64)); !q.Present || q.Lower.Cmp(first.Lower) != 0 {
		t.Fatal(q)
	}
	vb, e := r.BoundVolumes(ctx, AlgebraicTubeLensVolumeOptions{MaxDepth: 9})
	if e != nil || len(vb.Regions) != 2 {
		t.Fatal(vb, e)
	}
	want := math.Pi * 625 / (2 * 1024 * 1024)
	for i, x := range vb.Regions {
		lo, _ := x.Lower.Float64()
		hi, _ := x.Upper.Float64()
		if lo <= 0 || lo > want || hi < want {
			t.Fatalf("region %d volume [%g,%g] excludes %g", i, lo, hi, want)
		}
	}
	if r, e := s.SealAlgebraicTubeRegions(ctx, AlgebraicTubeRegionOptions{MaxLoops: 1}); e == nil || len(r.Regions()) != 0 {
		t.Fatal("partial region assembly after loop limit", e)
	}
}
