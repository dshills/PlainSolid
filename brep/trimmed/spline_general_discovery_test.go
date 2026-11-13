package trimmed

import (
	"context"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestAutomaticNonplanarTubeTerminalDiscovery(t *testing.T) {
	ctx := context.Background()
	build := func(second bool) intersect.Patch {
		k := []float64{0, 0, 1, 1}
		ku, degree := k, 1
		rows := []float64{0, 1}
		if !second {
			ku, degree = []float64{0, 0, 0, 1, 1, 1}, 2
			rows = []float64{0, .5, 1}
		}
		var p []geom.Vec3
		for _, u := range rows {
			for _, v := range []float64{0, 1} {
				if second {
					// X=s, Y=.5+.125*s*(2t-1), Z=2t-1.
					p = append(p, geom.Vec3{X: u, Y: .5 + .125*u*(2*v-1), Z: 2*v - 1})
				} else {
					x := .25 + .5*u
					z := 0.
					if u == .5 {
						z = -.03125
					}
					// The contact is a genuinely curved parabola in
					// model space with rational endpoints at z=0.
					p = append(p, geom.Vec3{X: x, Y: v, Z: z})
				}
			}
		}
		s, e := geom.NewBSplineSurface(degree, 1, len(rows), 2, p, ku, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return intersect.Patch{Surface: s, Max: geom.Vec2{X: 1, Y: 1}}
	}
	a, b := build(false), build(true)
	p, e := proposeGeneralSplineSections(ctx, a, b, [2]int{3, 7}, SplineShellOpenOptions{})
	if e != nil || len(p) != 1 || p[0].Closed || p[0].Start.Operand != 0 || p[0].End.Operand != 0 ||
		p[0].Start.Edge.Side == p[0].End.Edge.Side || len(p[0].TubeCells.Attempts) == 0 {
		t.Fatal(p, e)
	}
	trace, e := intersect.TraceSplinePair(ctx, a, b, intersect.SplinePairTraceOptions{})
	if e != nil || len(trace.Curves) != 1 || len(trace.Curves[0]) == 0 {
		t.Fatal(trace.Complete, e)
	}
	curves := trace.Curves[0]
	for end, segment := range [2]intersect.TubeSegment{curves[0], curves[len(curves)-1]} {
		q, e := proposeTerminalFromBoundaryContact(ctx, segment, end, a, b, SplineShellOpenOptions{})
		want := [2]SplineShellOpenEndpoint{p[0].Start, p[0].End}[end]
		if e != nil || !q.Proof.Proven || q.Operand != want.Operand || q.Side != want.Edge.Side {
			t.Fatal(q, want, e)
		}
		for i, x := range q.Proof.Parameters {
			if x.Cmp(want.Edge.Hint[i]) != 0 {
				t.Fatal(q.Proof.Parameters, want.Edge.Hint)
			}
		}
	}
}
