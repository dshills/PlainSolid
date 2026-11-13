package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestOverlappingGraphDisjointRegions(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1.0 / 64}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	xs := []float64{.0625, .28125, .5, .71875, .9375}
	ys := []float64{.25, .5, .75}
	var poles []geom.Vec3
	for i, x := range xs {
		xx := .21875 * .21875
		if i%2 == 1 {
			xx = -xx
		}
		for j, y := range ys {
			yy := .0625
			if j == 1 {
				yy = -yy
			}
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: 25.0/1024 - xx - yy + x*y/64})
		}
	}
	b, e := geom.NewBSplineSurface(2, 2, 5, 3, poles, []float64{.0625, .0625, .0625, .5, .5, .9375, .9375, .9375}, []float64{.25, .25, .25, .75, .75, .75}, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa := intersect.Patch{Surface: a, Min: geom.Vec2{X: .09375}, Max: geom.Vec2{X: .90625, Y: 1}}
	pb := intersect.Patch{Surface: b, Min: geom.Vec2{X: .0625, Y: .25}, Max: geom.Vec2{X: .9375, Y: .75}}
	o := intersect.OverlappingGraphTraceOptions{}
	o.Discovery.UCuts = []float64{0, .0625, .09375, .90625, .9375, 1, .375, .15625, .1875, .375, .40625, .59375, .625, .8125, .84375}
	o.Discovery.VCuts = []float64{0, .25, .75, 1, .375, .40625, .59375, .625}
	o.Discovery.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000

	patches := [2]intersect.Patch{pa, pb}
	var faces [2]Face
	for i, p := range patches {
		points := [4]geom.Vec2{p.Min, {X: p.Max.X, Y: p.Min.Y}, p.Max, {X: p.Min.X, Y: p.Max.Y}}
		faces[i].Surface = p.Surface
		for j, q := range points {
			faces[i].Outer = append(faces[i].Outer, Use{Edge: j, UVStart: q, UVEnd: points[(j+1)%4]})
		}
	}
	section := AlgebraicTrimmedPairSectionOptions{Trace: o.Trace}
	regions, e := SealOverlappingAffineGraphRegions(ctx, patches, faces, o.Discovery, section, AlgebraicTubeRegionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(regions.Regions()) != 2 || len(regions.Loops()) != 2 {
		t.Fatal("component count")
	}
	branches := map[int]bool{}
	for i, loop := range regions.Loops() {
		if loop.Parent != -1 || branches[loop.Branch] {
			t.Fatal("loop ancestry", loop)
		}
		branches[loop.Branch] = true
		if regions.Topology(i).EulerCharacteristic != 2 || regions.Topology(i).InnerWires != 0 {
			t.Fatal(regions.Topology(i))
		}
	}
	ids := map[int]bool{}
	for _, x := range []*big.Rat{big.NewRat(9, 32), big.NewRat(23, 32)} {
		q, e := regions.VerticalInterval(ctx, [2]*big.Rat{x, big.NewRat(1, 2)}, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !q.Complete || !q.Present || q.Region < 0 || ids[q.Region] {
			t.Fatal(q, e)
		}
		ids[q.Region] = true
		want := new(big.Rat).Quo(x, big.NewRat(128, 1))
		if q.Lower.Cmp(want) != 0 || new(big.Rat).Sub(q.Upper, q.Lower).Cmp(big.NewRat(25, 1024)) != 0 {
			t.Fatal(q)
		}
	}
	q, e := regions.VerticalInterval(ctx, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !q.Complete || q.Present {
		t.Fatal("between components", q, e)
	}
	var data bytes.Buffer
	if e = WriteAlgebraicTubeRegionsJSON(&data, regions); e != nil {
		t.Fatal(e)
	}
	section.Trace.Coverage.OverlappingAffineXYFallback = true
	reload, e := ReadAlgebraicTubeRegionsJSON(ctx, &data, JSONLimits{}, section, AlgebraicTubeRegionOptions{})
	if e != nil || len(reload.Regions()) != 2 {
		t.Fatal("native reload", e)
	}
	limited, e := SealOverlappingAffineGraphRegions(ctx, patches, faces, o.Discovery, section, AlgebraicTubeRegionOptions{MaxLoops: 1})
	if e == nil || len(limited.Regions()) != 0 {
		t.Fatal("region budget returned partial material", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	empty, e := SealOverlappingAffineGraphRegions(canceled, patches, faces, o.Discovery, section, AlgebraicTubeRegionOptions{})
	if !errors.Is(e, context.Canceled) || len(empty.Regions()) != 0 {
		t.Fatal(e)
	}

}
