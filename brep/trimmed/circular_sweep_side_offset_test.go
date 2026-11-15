package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func circularSweepOffsetFixture() ExactCircularSweep {
	return ExactCircularSweep{
		Outer:  exactLoopPolygon(geom.Vec2{}, geom.Vec2{X: 8}, geom.Vec2{X: 8, Y: 6}, geom.Vec2{Y: 6}),
		Holes:  [][]ExactCircularLoopSegment{exactLoopQuarterDisk(geom.Vec2{X: 1, Y: 1}, .5, true)},
		Bottom: -2, Top: 0,
	}
}

func TestExactCircularSweepSideOffsetPreservesRingsAndCapElevations(t *testing.T) {
	ctx := context.Background()
	h := circularSweepOffsetFixture()
	h.Holes = append(h.Holes, exactLoopQuarterDisk(geom.Vec2{X: 6, Y: 4}, .5, true))
	for _, d := range []float64{.125, -.125, 0} {
		r, err := OffsetExactCircularSweepSide(ctx, h, d, CircularSweepOffsetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Profile.Bottom != h.Bottom || r.Profile.Top != h.Top || len(r.Profile.Holes) != 2 ||
			r.Profile.Outer[0].Start != (geom.Vec2{X: -d, Y: -d}) ||
			r.Profile.Holes[0][0].Center != h.Holes[0][0].Center ||
			r.Profile.Holes[1][0].Radius != .5-d || len(r.Solid.Faces()) != 14 || (d != 0 && r.CheckedPairBoxes == 0) {
			t.Fatalf("offset lost original topology or profile: %+v", r.Profile)
		}
		w, v := 8+2*d, 6+2*d
		bore := .5 - d
		foot := w*v - 2*math.Pi*bore*bore
		perim := 2*(w+v) + 4*math.Pi*bore
		m, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if err != nil || math.Abs(m.Volume-2*foot) > 1e-7 || math.Abs(m.Area-(2*foot+2*perim)) > 1e-7 {
			t.Fatalf("offset measures differ for %v: %+v %v", d, m, err)
		}
	}
}

func TestExactCircularSweepSideOffsetReentrantTangentArc(t *testing.T) {
	ctx := context.Background()
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	outer := exactLoopPolygon(p(0, 0), p(4, 0), p(4, 1), p(2, 1), p(1, 2), p(1, 4), p(0, 4))
	outer[3].Center, outer[3].Radius = p(2, 2), 1
	source := ExactCircularSweep{Outer: outer, Bottom: -1, Top: 1}
	for _, d := range []float64{.125, -.125} {
		got, err := OffsetExactCircularSweepSide(ctx, source, d, CircularSweepOffsetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		arc := got.Profile.Outer[3]
		if arc.Center != p(2, 2) || arc.Radius != 1-d || arc.Start != p(2, 1+d) || arc.End != p(1+d, 2) {
			t.Fatalf("reentrant offset changed center, direction or exact tangent vertices: %+v", arc)
		}
		side, arm, r := 4+2*d, 1+2*d, 1-d
		foot := 2*side*arm - arm*arm + r*r*(1-math.Pi/4)
		perim := 4*side + r*(math.Pi/2-2)
		m, err := got.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if err != nil || math.Abs(m.Volume-2*foot) > 1e-7 || math.Abs(m.Area-(2*foot+2*perim)) > 1e-7 {
			t.Fatalf("reentrant offset differs for %v: %+v %v", d, m, err)
		}
	}
}

func TestExactCircularSweepSideOffsetRejectsUnprovedMotion(t *testing.T) {
	ctx := context.Background()
	source := circularSweepOffsetFixture()
	for _, tc := range []struct {
		name    string
		offset  float64
		options CircularSweepOffsetOptions
		want    error
	}{
		{"missing separation budget", .125, CircularSweepOffsetOptions{MaxPairBoxes: 1}, geom.ErrConvergence},
		{"collapsed circle", .5, CircularSweepOffsetOptions{}, geom.ErrDomain},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := OffsetExactCircularSweepSide(ctx, source, tc.offset, tc.options)
			if !errors.Is(err, tc.want) || len(r.Solid.Faces()) != 0 {
				t.Fatalf("offset published unproved material: %v", err)
			}
		})
	}
	source.Holes = append(source.Holes, exactLoopQuarterDisk(geom.Vec2{X: 2.125, Y: 1}, .5, true))
	if got, err := OffsetExactCircularSweepSide(ctx, source, -.125, CircularSweepOffsetOptions{}); !errors.Is(err, geom.ErrConvergence) || len(got.Solid.Faces()) != 0 {
		t.Fatalf("offset accepted colliding original holes: %v", err)
	}
}
