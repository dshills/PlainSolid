package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func checkExactSplitBox(t *testing.T, s Solid, vertices, edges int) {
	t.Helper()
	if len(s.Vertices()) != vertices || len(s.Edges()) != edges || len(s.Faces()) != 6 || s.Topology().EulerCharacteristic != 2 {
		t.Fatalf("unexpected split topology: V%d E%d F%d", len(s.Vertices()), len(s.Edges()), len(s.Faces()))
	}
	proof, e := s.ValidateEdgeAgreement(context.Background(), edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatalf("edge agreement: %+v, %v", proof, e)
	}
	p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-1) > 1e-12 || math.Abs(p.Area-6) > 1e-12 {
		t.Fatalf("box mass: %+v, %v", p, e)
	}
}

func TestSplitSplineEdgeExactReversedNonunitRepeated(t *testing.T) {
	ctx := context.Background()
	s, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	old := edges[0]
	// Reverse the model parameter direction independently of both incident
	// face charts, and give the model curve the nonunit domain [2,6].
	start, end := vertices[old.End], vertices[old.Start]
	curve, e := geom.NewBSpline(1, []geom.Vec3{start, end}, []float64{2, 2, 6, 6}, nil)
	if e != nil {
		t.Fatal(e)
	}
	edges[0] = parametric.Edge{Start: old.End, End: old.Start, Curve: curve, Range: parametric.Interval{Min: 2, Max: 6}}
	for i := range faces {
		for j := range faces[i].Outer {
			if faces[i].Outer[j].Edge == 0 {
				faces[i].Outer[j].Reversed = !faces[i].Outer[j].Reversed
			}
		}
	}
	s, e = NewSolidContext(ctx, vertices, edges, faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	checkExactSplitBox(t, s, 8, 12)
	out, h, e := s.SplitSplineEdgeExact(ctx, 0, 3, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if out.vertices[h.Vertex] != start.Add(end.Sub(start).Scale(.25)) {
		t.Fatal("wrong first split point", out.vertices[h.Vertex])
	}
	checkExactSplitBox(t, out, 9, 13)
	// Split the retained nonunit subrange [3,6], then the first subrange [2,3].
	out, h2, e := out.SplitSplineEdgeExact(ctx, h.Second, 4, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if out.vertices[h2.Vertex] != start.Add(end.Sub(start).Scale(.5)) {
		t.Fatal("wrong repeated split point", out.vertices[h2.Vertex])
	}
	out, h3, e := out.SplitSplineEdgeExact(ctx, h.First, 2.5, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if out.vertices[h3.Vertex] != start.Add(end.Sub(start).Scale(.125)) {
		t.Fatal("wrong first-subrange point", out.vertices[h3.Vertex])
	}
	checkExactSplitBox(t, out, 11, 15)
	checkExactSplitBox(t, s, 8, 12) // caller-owned input is unchanged.
}

func TestSplitSplineEdgeExactBudgets(t *testing.T) {
	s := testWarpedSplinePrism(t, false, false)
	edge := s.edges[0]
	parameter := (edge.Range.Min + edge.Range.Max) / 2
	for _, o := range []edgecheck.Options{{MaxWork: 1}, {MaxBits: 1}, {MaxWork: -1}, {MaxBits: -1}} {
		out, h, e := s.SplitSplineEdgeExact(context.Background(), 0, parameter, o)
		if e == nil {
			t.Fatalf("accepted insufficient/invalid budget %+v", o)
		}
		if len(out.Vertices()) != 0 || h != (EdgeSplit{}) {
			t.Fatalf("partial result on budget failure %+v", o)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e := s.SplitSplineEdgeExact(ctx, 0, parameter, edgecheck.Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	out, _, e := s.SplitSplineEdgeExact(context.Background(), 0, parameter, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := out.ValidateEdgeAgreement(context.Background(), edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal(proof, e)
	}
	before, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	after, e := out.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(before.Area-after.Area) > 1e-10 || math.Abs(before.Volume-after.Volume) > 1e-10 {
		t.Fatal(before, after, e)
	}
}

func TestSplitSplineEdgeExactReverseIsoSubrange(t *testing.T) {
	ctx := context.Background()
	s, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	id := -1
	for i, edge := range s.edges {
		_, reverse, e := exactEdgeSpline(edge)
		if e != nil {
			t.Fatal(e)
		}
		if reverse {
			id = i
			break
		}
	}
	if id < 0 {
		t.Fatal("fixture lacks decreasing isoparametric model curve")
	}
	a, b := s.vertices[s.edges[id].Start], s.vertices[s.edges[id].End]
	out, h, e := s.SplitSplineEdgeExact(ctx, id, .25, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	out, h2, e := out.SplitSplineEdgeExact(ctx, h.Second, .75, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if out.vertices[h.Vertex] != a.Add(b.Sub(a).Scale(.25)) || out.vertices[h2.Vertex] != a.Add(b.Sub(a).Scale(.75)) {
		t.Fatal("decreasing isocurve subrange mapped incorrectly")
	}
	checkExactSplitBox(t, out, 10, 14)
}
