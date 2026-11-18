package intersect

import (
	"context"
	"errors"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func TestCertifyOverlappingProjectedGraphPair(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	x, z := [3]float64{0, .5, 1}, [3]float64{.25, -.25, .25}
	var poles []geom.Vec3
	for i := range x {
		for j := range x {
			poles = append(poles, geom.Vec3{X: x[i] + .125, Y: x[j], Z: z[i] + z[j] - 25.0/256})
		}
	}
	knot := []float64{0, 0, 0, 1, 1, 1}
	b, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knot, knot, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}
	o := SplinePairTraceOptions{}
	o.AffineGraphFallback.Enabled = true
	o.AffineGraphFallback.UCuts = []float64{.375, .4375, .8125, .875}
	o.AffineGraphFallback.VCuts = []float64{.25, .3125, .6875, .75}
	o.AffineGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Coverage.AffineXYFallback = true
	o.Coverage.Isolation.SeparatingProjections = true
	o.Coverage.Isolation.ParameterTolerance = .25
	o.Coverage.Cell.MaxOperations = 16000000

	for _, axes := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
		t.Run(string(rune('X'+axes[0]))+string(rune('X'+axes[1])), func(t *testing.T) {
			order := [3]int{}
			order[axes[0]] = 0
			order[axes[1]] = 1
			order[3-axes[0]-axes[1]] = 2
			sa, err := a.PermutedModelAxes(order)
			if err != nil {
				t.Fatal(err)
			}
			sb, err := b.PermutedModelAxes(order)
			if err != nil {
				t.Fatal(err)
			}
			aa, bb := pa, pb
			aa.Surface = sa
			bb.Surface = sb
			options := OverlappingGraphTraceOptions{Discovery: o.AffineGraphFallback, Trace: o}
			result, err := CertifyOverlappingProjectedGraphPair(ctx, aa, bb, axes, options)
			if err != nil || !result.Complete || result.Disjoint || len(result.Trace.Curves) != 1 || !result.Trace.BranchGraph.Branches[0].Closed {
				t.Fatal(result.Complete, err)
			}
			if result.A.Surface != sa || result.B.Surface != sb || result.A.Min.X != .125 || result.B.Max.X != .875 {
				t.Fatal("lost original coordinates")
			}
			options.Discovery.MaxTubes = 1
			bad, err := CertifyOverlappingProjectedGraphPair(ctx, aa, bb, axes, options)
			if err == nil || bad.Complete || bad.A.Surface != nil {
				t.Fatal("partial result", err)
			}
		})
	}
	small := pa
	small.Max.X = .0625
	empty, err := CertifyOverlappingAffineXYGraphPair(ctx, small, pb, OverlappingGraphTraceOptions{})
	if err != nil || !empty.Complete || !empty.Disjoint || empty.A.Surface != nil {
		t.Fatal("strict disjoint", empty, err)
	}
	small.Max.X = .125
	touching, err := CertifyOverlappingAffineXYGraphPair(ctx, small, pb, OverlappingGraphTraceOptions{})
	if err == nil || touching.Complete {
		t.Fatal("touch must remain unresolved", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = CertifyOverlappingAffineXYGraphPair(canceled, pa, pb, OverlappingGraphTraceOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	_, e = CertifyOverlappingProjectedGraphPair(ctx, pa, pb, [2]int{0, 0}, OverlappingGraphTraceOptions{})
	if !errors.Is(e, geom.ErrInvalid) {
		t.Fatal(e)
	}
}
