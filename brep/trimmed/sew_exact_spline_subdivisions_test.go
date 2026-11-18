//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func nonconformingSplineSewFixture(t *testing.T) (Solid, Solid, int, int, int) {
	t.Helper()
	ctx := context.Background()
	closed, _ := crossingFootprintRoofShellShape(t, false, true)
	vertices, edges, faces := explodeExactSewSource(closed)
	f := &faces[1]
	for j, u := range f.Outer {
		e := edges[u.Edge]
		a, b := vertices[e.Start], vertices[e.End]
		if e.Range != (parametric.Interval{Max: 1}) || u.Curve != nil {
			continue
		}
		descriptor, _, err := exactSewCurve(ctx, e, [2]geom.Vec3{a, b}, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		lo, hi := descriptor.Domain()
		middle, err := descriptor.ExactPoint((lo + hi) / 2)
		if err != nil {
			t.Fatal(err)
		}
		if middle == a.Add(b).Scale(.5) {
			continue
		}
		coarse := -1
		for i, other := range edges {
			if i != u.Edge && vertices[other.Start] == a && vertices[other.End] == b {
				coarse = i
				break
			}
		}
		if coarse < 0 {
			t.Fatal("exact curved coarse owner absent")
		}
		v, second := len(vertices), len(edges)
		vertices = append(vertices, middle)
		firstHalf, secondHalf := e, e
		firstHalf.End, firstHalf.Range.Max = v, .5
		secondHalf.Start, secondHalf.Range.Min = v, .5
		edges[u.Edge] = firstHalf
		edges = append(edges, secondHalf)
		uv := geom.Vec2{X: (u.UVStart.X + u.UVEnd.X) / 2, Y: (u.UVStart.Y + u.UVEnd.Y) / 2}
		before, after := u, u
		before.UVEnd, after.UVStart = uv, uv
		if u.Reversed {
			before.Edge = second
		} else {
			after.Edge = second
		}
		wire := append([]Use(nil), f.Outer[:j]...)
		wire = append(wire, before, after)
		f.Outer = append(wire, f.Outer[j+1:]...)
		return closed, Solid{vertices: vertices, edges: edges, faces: faces}, coarse, u.Edge, second
	}
	t.Fatal("fixture has no independently divided curved spline border")
	return Solid{}, Solid{}, 0, 0, 0
}

func TestSewExactSplineSubdivisions(t *testing.T) {
	ctx := context.Background()
	baseline, input, coarse, first, second := nonconformingSplineSewFixture(t)
	failed, _, err := SewExactStraightSubdivisions(ctx, input.vertices, input.edges, input.faces, ExactStraightSewOptions{})
	if !errors.Is(err, geom.ErrConvergence) || len(failed.faces) != 0 {
		t.Fatal("straight source proof unexpectedly accepted a curved split", err)
	}
	got, report, err := SewExactSplineSubdivisions(ctx, input.vertices, input.edges, input.faces, ExactSplineSewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if g, b := got.Topology(), baseline.Topology(); g.EulerCharacteristic != 2 || g.Faces != b.Faces || g.Vertices != b.Vertices+1 || g.Edges != b.Edges+1 ||
		report.InsertedVertices != 1 || len(report.Vertices) != len(input.vertices) || report.DiscoveryWork <= 0 {
		t.Fatal("incomplete source and native proof", g, report)
	}
	segments := report.Segments[coarse]
	if len(segments) != 2 || segments[0].Upper.Cmp(big.NewRat(1, 2)) != 0 || segments[0].Upper.Cmp(segments[1].Lower) != 0 ||
		segments[0].Edge != report.Segments[first][0].Edge || segments[1].Edge != report.Segments[second][0].Edge {
		t.Fatal("original exact restriction ancestry mismatch", segments)
	}
	for i, face := range got.faces {
		if face.Surface != baseline.faces[i].Surface {
			t.Fatal("source spline support was replaced", i)
		}
	}
	var native bytes.Buffer
	if err = WriteJSON(&native, got); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadJSONContext(ctx, &native, JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	agreement, err := reloaded.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !agreement.Complete {
		t.Fatal("reloaded spline border failed continuous proof", err, agreement.Complete)
	}
	baseMass, err := baseline.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mass, err := reloaded.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(mass.Volume-baseMass.Volume) > 1e-9 {
		t.Fatal("sewing changed physical volume", mass.Volume, baseMass.Volume, err)
	}
	exactLimit := ExactSplineSewOptions{MaxDiscoveryWork: report.DiscoveryWork, MaxPairTests: report.PairTests}
	if _, _, err := SewExactSplineSubdivisions(ctx, input.vertices, input.edges, input.faces, exactLimit); err != nil {
		t.Fatal("the exact reported discovery limits should suffice", err)
	}
	exactLimit.MaxDiscoveryWork--
	if s, r, err := SewExactSplineSubdivisions(ctx, input.vertices, input.edges, input.faces, exactLimit); !errors.Is(err, geom.ErrConvergence) || len(s.faces) != 0 || len(r.Segments) != 0 {
		t.Fatal("one-short discovery limit returned a partial solid", err, s.Topology(), r)
	}
}

func TestExactSplineSewEndpointAnchorsRequireFullOverlap(t *testing.T) {
	ctx := context.Background()
	makeBorder := func(points []geom.Vec3) splineSewBorder {
		t.Helper()
		knots := []float64{0, 0, 0, 1, 1, 1}
		curve, err := geom.NewBSpline(2, points, knots, nil)
		if err != nil {
			t.Fatal(err)
		}
		exact, err := geom.NewExactSplineCurve(curve, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		bounds := [3][2]*big.Rat{{new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), new(big.Rat)}}
		return splineSewBorder{curve: exact, ends: [2]geom.Vec3{points[0], points[2]}, bounds: bounds, preimage: make(map[geom.Vec3]splineSewPreimage)}
	}
	a := makeBorder([]geom.Vec3{{}, {X: .5, Y: .5}, {X: 1}})
	for _, middle := range []float64{.3125, .375} {
		b := makeBorder([]geom.Vec3{{X: .25, Y: .1875}, {X: .5, Y: middle}, {X: .75, Y: .1875}})
		d := splineSewDiscovery{ctx: ctx, options: ExactSplineSewOptions{MaxDiscoveryWork: 100000}}
		interval, err := d.overlap(&a, &b)
		if err != nil {
			t.Fatal(err)
		}
		if middle == .3125 {
			if interval[0][0] == nil || interval[0][0].Cmp(big.NewRat(1, 4)) != 0 || interval[0][1].Cmp(big.NewRat(3, 4)) != 0 {
				t.Fatal("exact spline restriction failed", interval)
			}
		} else if interval[0][0] != nil || interval[1][0] != nil {
			t.Fatal("two true endpoint preimages hid a different curve interior", interval)
		}
	}
}
