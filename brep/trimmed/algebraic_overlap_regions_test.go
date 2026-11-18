package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestOverlappingGraphDisjointRegions(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1.0 / 64}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	aPatch, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	data, e := aPatch.ExportExactBezierRecipe()
	if e != nil {
		t.Fatal(e)
	}
	// Degree elevation in U keeps the bilinear graph unchanged and
	// introduces genuinely nonbinary homogeneous coefficients.
	net := make([][4]string, 8)
	for i := int64(0); i <= 3; i++ {
		for j := range 2 {
			for c := range 4 {
				l, _ := new(big.Rat).SetString(data.Controls[j][c])
				h, _ := new(big.Rat).SetString(data.Controls[j+2][c])
				q := new(big.Rat).Add(new(big.Rat).Mul(l, big.NewRat(3-i, 3)), new(big.Rat).Mul(h, big.NewRat(i, 3)))
				net[int(i)*2+j][c] = q.RatString()
			}
		}
	}
	if net[2][0] != "1/3" {
		t.Fatal("missing nonbinary control")
	}
	data.DegreeU, data.Controls = 3, net
	aPatch, e = geom.ImportExactBezierSurfaceRecipe(ctx, data, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	aExact, e := geom.NewExactBezierSurface(aPatch)
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
	var native bytes.Buffer
	if e = WriteAlgebraicTubeRegionsJSON(&native, regions); e != nil {
		t.Fatal(e)
	}
	section.Trace.Coverage.OverlappingAffineXYFallback = true
	reload, e := ReadAlgebraicTubeRegionsJSON(ctx, &native, JSONLimits{}, section, AlgebraicTubeRegionOptions{})
	if e != nil || len(reload.Regions()) != 2 {
		t.Fatal("native reload", e)
	}
	// Keep the original unequal rectangles and substitute the exact
	// owner. Full coverage and loop nesting must be re-proved.
	patches[0].Surface, faces[0].Surface = aExact, aExact
	exact, e := SealOverlappingAffineGraphRegions(ctx, patches, faces, o.Discovery, section, AlgebraicTubeRegionOptions{})
	if e != nil {
		t.Fatal("exact source reproof", e)
	}
	v, e := exact.BoundVolumes(ctx, AlgebraicTubeLensVolumeOptions{MaxDepth: 9})
	if e != nil || len(v.Regions) != 2 {
		t.Fatal("exact region bounds", v, e)
	}
	// Each paraboloid has radius 5/32, with volume πR⁴/2.
	low := new(big.Rat).Mul(big.NewRat(625, 2*1024*1024), big.NewRat(333, 106))
	high := new(big.Rat).Mul(big.NewRat(625, 2*1024*1024), big.NewRat(355, 113))
	for i, q := range v.Regions {
		if q.Lower.Sign() <= 0 || q.Lower.Cmp(low) > 0 || q.Upper.Cmp(high) < 0 {
			t.Fatal("independent lobe volume", i, q, low, high)
		}
	}
	if partial, e := exact.BoundVolumes(ctx, AlgebraicTubeLensVolumeOptions{MaxCells: 1}); e == nil || len(partial.Regions) != 0 {
		t.Fatal("cell budget leaked partial region values", partial, e)
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
