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

func exactMultispanSewSource(t *testing.T, width float64) (Solid, []*geom.BSplineSurface) {
	t.Helper()
	var surfaces []*geom.BSplineSurface
	var reverse []bool
	breaks := [3][]float64{{0, width / 2, width}, {0, 1}, {0, 1}}
	knots := [3][]float64{{0, 0, width / 2, width, width}, {0, 0, 1, 1}, {0, 0, 1, 1}}
	for axis := range 3 {
		u, v := (axis+1)%3, (axis+2)%3
		for _, side := range []float64{0, breaks[axis][len(breaks[axis])-1]} {
			var net []geom.Vec3
			for _, a := range breaks[u] {
				for _, b := range breaks[v] {
					var p [3]float64
					p[axis], p[u], p[v] = side, a, b
					if p[0] == width/2 {
						p[2] += width / 8
					}
					net = append(net, geom.Vec3{X: p[0], Y: p[1], Z: p[2]})
				}
			}
			s, e := geom.NewBSplineSurface(1, 1, len(breaks[u]), len(breaks[v]), net, knots[u], knots[v], nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces = append(surfaces, s)
			reverse = append(reverse, side == 0)
		}
	}
	baseline, _, e := SewSplineRectangles(context.Background(), surfaces, make([]geom.ParameterRectangle, len(surfaces)), reverse, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal("exact warped rectangular input", e)
	}
	return baseline, surfaces
}

func TestSewExactMultispanSplineSubdivisions(t *testing.T) {
	ctx := context.Background()
	baseline, surfaces := exactMultispanSewSource(t, 1)
	for _, cut := range []float64{.5, .75} {
		vertices, edges, faces := explodeExactSewSource(baseline)
		// Original roof edge follows x across a genuine C0 knot. It is
		// independently split on the roof while the side keeps its full range.
		const roof = 5
		var coarse, first, second int
		for j, use := range faces[roof].Outer {
			edge := edges[use.Edge]
			a, b := vertices[edge.Start], vertices[edge.End]
			if a.X == b.X {
				continue
			}
			first, second, coarse = use.Edge, len(edges), -1
			for i, candidate := range edges {
				if i != first && vertices[candidate.Start] == a && vertices[candidate.End] == b {
					coarse = i
					break
				}
			}
			if coarse < 0 || edge.Range != (parametric.Interval{Max: 1}) {
				t.Fatal("missing original whole opposing coedge")
			}
			span, backward, e := exactSewCurve(ctx, edge, [2]geom.Vec3{a, b}, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if _, _, e := span.BezierControlNet(ctx, geom.SurfacePatchOptions{}); !errors.Is(e, geom.ErrConvergence) {
				t.Fatal("source border must have a genuine interior knot", e)
			}
			lo, hi := span.Domain()
			toward := cut
			if use.Reversed {
				toward = 1 - toward
			}
			modelParameter := cut
			if backward {
				modelParameter = 1 - cut
			}
			mid, e := span.ExactPoint(lo + (hi-lo)*modelParameter)
			if e != nil || mid.Z <= 1 {
				t.Fatal("split point must follow curved source border", mid, e)
			}
			id := len(vertices)
			vertices = append(vertices, mid)
			loEdge, hiEdge := edge, edge
			loEdge.End, loEdge.Range.Max = id, cut
			hiEdge.Start, hiEdge.Range.Min = id, cut
			edges[first] = loEdge
			edges = append(edges, hiEdge)
			uv := geom.Vec2{
				X: use.UVStart.X + toward*(use.UVEnd.X-use.UVStart.X),
				Y: use.UVStart.Y + toward*(use.UVEnd.Y-use.UVStart.Y),
			}
			before, after := use, use
			before.UVEnd, after.UVStart = uv, uv
			if use.Reversed {
				before.Edge = second
			} else {
				after.Edge = second
			}
			faces[roof].Outer = append(append(append([]Use(nil), faces[roof].Outer[:j]...), before, after), faces[roof].Outer[j+1:]...)
			break
		}
		if second == 0 {
			t.Fatal("missing independently subdivided source face")
		}
		if old, _, e := SewExactSplineSubdivisions(ctx, vertices, edges, faces, ExactSplineSewOptions{}); e == nil || len(old.faces) != 0 {
			t.Fatal("expected the explicit multispan subdivision proof", e)
		}
		out, report, e := SewExactSplineSubdivisions(ctx, vertices, edges, faces, ExactSplineSewOptions{MultiSpan: true})
		if e != nil || report.InsertedVertices != 1 || out.Topology().EulerCharacteristic != 2 {
			t.Fatal("exact multispan sew", cut, report, e)
		}
		segments := report.Segments[coarse]
		if len(segments) != 2 || segments[0].Upper.Cmp(new(big.Rat).SetFloat64(cut)) != 0 ||
			report.Segments[first][0].Edge != segments[0].Edge || report.Segments[second][0].Edge != segments[1].Edge ||
			out.faces[roof].Surface != surfaces[roof] {
			t.Fatal("original closed seam ranges and supports", cut, segments)
		}
		var native bytes.Buffer
		if e := WriteJSON(&native, out); e != nil {
			t.Fatal(e)
		}
		out, e = ReadJSONContext(ctx, &native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		p, e := out.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-1) > 1e-9 {
			t.Fatal("piecewise vertical translation preserves unit volume", p, e)
		}
		agreement, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !agreement.Complete {
			t.Fatal("continuous original multispan coedge agreement", agreement, e)
		}
		if cut == .75 {
			options := ExactSplineSewOptions{MultiSpan: true, MaxDiscoveryWork: report.DiscoveryWork, MaxPairTests: report.PairTests}
			if _, _, e := SewExactSplineSubdivisions(ctx, vertices, edges, faces, options); e != nil {
				t.Fatal("reported discovery and pair budgets suffice", e)
			}
			options.MaxDiscoveryWork--
			if s, report, e := SewExactSplineSubdivisions(ctx, vertices, edges, faces, options); !errors.Is(e, geom.ErrConvergence) ||
				len(s.faces) != 0 || len(report.Segments) != 0 {
				t.Fatal("one-short multispan discovery budget must fail atomically", e)
			}
		}
	}
}

func TestSewExactMultispanRationalModelPreimage(t *testing.T) {
	ctx := context.Background()
	stock, supports := exactMultispanSewSource(t, 3)
	vertices, edges, faces := explodeExactSewSource(stock)
	roof := &faces[5]
	for j, use := range roof.Outer {
		edge := edges[use.Edge]
		a, b := vertices[edge.Start], vertices[edge.End]
		if a.X == b.X {
			continue
		}
		coarse := -1
		for k, neighbor := range edges {
			if k != use.Edge && vertices[neighbor.Start] == a && vertices[neighbor.End] == b {
				coarse = k
				break
			}
		}
		if coarse < 0 {
			t.Fatal("whole original model edge")
		}
		model, reversal, e := exactSewCurve(ctx, edges[coarse], [2]geom.Vec3{a, b}, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		model, e = model.Reparameterize(0, 1, reversal, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		edges[coarse].Curve = model
		uv := geom.Vec2{X: 1, Y: use.UVStart.Y}
		id, second := len(vertices), len(edges)
		vertices = append(vertices, geom.Vec3{X: 1, Y: a.Y, Z: 1.25})
		start, end := use.UVStart, use.UVEnd
		if use.Reversed {
			start, end = end, start
		}
		range01 := parametric.Interval{Max: 1}
		edges[use.Edge] = parametric.Edge{Start: edge.Start, End: id, Range: range01, Curve: surfaceTrimCurve{surface: roof.Surface, a: start, b: uv}}
		edges = append(edges, parametric.Edge{Start: id, End: edge.End, Range: range01, Curve: surfaceTrimCurve{surface: roof.Surface, a: uv, b: end}})
		x, y := use, use
		x.UVEnd, y.UVStart = uv, uv
		if use.Reversed {
			x.Edge = second
		} else {
			y.Edge = second
		}
		roof.Outer = append(append(append([]Use(nil), roof.Outer[:j]...), x, y), roof.Outer[j+1:]...)

		s, report, e := SewExactSplineSubdivisions(ctx, vertices, edges, faces, ExactSplineSewOptions{MultiSpan: true})
		if e != nil || s.Topology().EulerCharacteristic != 2 || report.InsertedVertices != 1 || s.faces[5].Surface != supports[5] {
			t.Fatal("native exact split from a nonbinary model parameter", e)
		}
		want := big.NewRat(1, 3)
		if a.X > b.X {
			want = big.NewRat(2, 3)
		}
		if split := report.Segments[coarse]; len(split) != 2 || split[0].Upper.Cmp(want) != 0 || split[1].Lower.Cmp(want) != 0 {
			t.Fatal("nonbinary original model range lost", report.Segments[coarse])
		}
		var data bytes.Buffer
		if e := WriteJSON(&data, s); e != nil {
			t.Fatal(e)
		}
		s, e = ReadJSONContext(ctx, &data, JSONLimits{})
		if e != nil || s.Topology().EulerCharacteristic != 2 {
			t.Fatal("original support and sewn partition round trip", e)
		}
		return
	}
	t.Fatal("no original edge crossing the roof knot")
}

func TestExactMultispanSewRejectsChangedInnerKnot(t *testing.T) {
	ctx := context.Background()
	border := func(a, b, c geom.Vec3) splineSewBorder {
		t.Helper()
		s, e := geom.NewBSpline(1, []geom.Vec3{a, b, c}, []float64{0, 0, .5, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		x, e := geom.NewExactSplineCurve(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return splineSewBorder{curve: x, ends: [2]geom.Vec3{a, c}, preimage: make(map[geom.Vec3]splineSewPreimage), multispan: true,
			bounds: [3][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(0, 1)}}}
	}
	a := border(geom.Vec3{}, geom.Vec3{X: .5, Y: .5}, geom.Vec3{X: 1})
	for _, height := range []float64{.5, .625} {
		// The opposing original curve is traversed in the reverse direction.
		b := border(geom.Vec3{X: .75, Y: .25}, geom.Vec3{X: .5, Y: height}, geom.Vec3{X: .25, Y: .25})
		d := splineSewDiscovery{ctx: ctx, options: ExactSplineSewOptions{MultiSpan: true, MaxDiscoveryWork: 100000}}
		span, e := d.overlap(&a, &b)
		if e != nil || (span[0][0] != nil) != (height == .5) {
			t.Fatal("certified endpoints cannot mask a changed interior knot", height, span, e)
		}
	}
}
