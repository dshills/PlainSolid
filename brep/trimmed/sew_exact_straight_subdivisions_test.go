package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func nonconformingStraightSewFixture(t *testing.T) (Solid, int, int, int) {
	t.Helper()
	p, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 3, Y: 2, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	closed, err := FromPlanar(p, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	v, edges, faces := explodeExactSewSource(closed)
	for fi, face := range faces {
		for ui, use := range face.Outer {
			edge := edges[use.Edge]
			start, end := v[edge.Start], v[edge.End]
			if !use.Reversed || math.Abs(start.X-end.X) != 3 {
				continue
			}
			coarse := -1
			for j, other := range edges {
				if j != use.Edge && v[other.Start] == start && v[other.End] == end {
					coarse = j
					break
				}
			}
			if coarse < 0 {
				t.Fatal("coarse answer edge absent")
			}
			point, err := edge.Curve.Eval(1)
			if err != nil {
				t.Fatal(err)
			}
			middle, other := len(v), len(edges)
			v = append(v, point)
			first, second := edge, edge
			first.End, first.Range.Max = middle, 1
			second.Start, second.Range.Min = middle, 1
			edges[use.Edge] = first
			edges = append(edges, second)
			x, xok := exactRectEdgeMix(big.NewRat(2, 3), use.UVStart.X, use.UVEnd.X)
			y, yok := exactRectEdgeMix(big.NewRat(2, 3), use.UVStart.Y, use.UVEnd.Y)
			if !xok || !yok {
				t.Fatal("fixture UV subdivision must be exact")
			}
			uv := geom.Vec2{X: x, Y: y}
			a, b := use, use
			a.Edge, a.UVEnd, b.UVStart = other, uv, uv
			loop := append([]Use(nil), face.Outer[:ui]...)
			loop = append(loop, a, b)
			faces[fi].Outer = append(loop, face.Outer[ui+1:]...)
			return Solid{vertices: v, edges: edges, faces: faces}, coarse, use.Edge, other
		}
	}
	t.Fatal("fixture requires a reversed border on a length-three axis")
	return Solid{}, 0, 0, 0
}

func TestSewExactStraightSubdivisions(t *testing.T) {
	ctx := context.Background()
	input, coarse, smallA, smallB := nonconformingStraightSewFixture(t)
	if _, _, err := SewExactTopology(ctx, input.vertices, input.edges, input.faces, ExactSewOptions{}); !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("ordinary exact sewing should detect the ungapped but unmatched sides", err)
	}
	result, report, err := SewExactStraightSubdivisions(ctx, input.vertices, input.edges, input.faces, ExactStraightSewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if top := result.Topology(); top.Vertices != 9 || top.Edges != 13 || top.Faces != 6 || top.EulerCharacteristic != 2 ||
		report.InsertedVertices != 1 || len(report.Vertices) != len(input.vertices) || len(report.FlippedFaces) != 0 {
		t.Fatal("invalid native graph or ancestry", top, report)
	}
	segs := report.Segments[coarse]
	if len(segs) != 2 || segs[0].Upper.Cmp(big.NewRat(1, 3)) != 0 || segs[0].Upper.Cmp(segs[1].Lower) != 0 ||
		segs[0].Edge != report.Segments[smallA][0].Edge || segs[1].Edge != report.Segments[smallB][0].Edge {
		t.Fatal("lost the exact nonbinary source parameter or pairing", segs)
	}
	for i, edge := range input.edges {
		pieces := report.Segments[i]
		first, last := pieces[0], pieces[len(pieces)-1]
		a, b := result.edges[first.Edge], result.edges[last.Edge]
		beg, end := a.Start, b.End
		if first.Reversed {
			beg = a.End
		}
		if last.Reversed {
			end = b.Start
		}
		if report.Vertices[edge.Start] != beg || report.Vertices[edge.End] != end {
			t.Fatal("original vertex ownership mismatch", i, pieces)
		}
	}
	for i, f := range input.faces {
		if result.faces[i].Surface != f.Surface {
			t.Fatal("original surface was replaced", i)
		}
	}
	var raw bytes.Buffer
	if err := WriteJSON(&raw, result); err != nil {
		t.Fatal(err)
	}
	got, err := ReadJSONContext(ctx, &raw, JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	proof, err := got.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete {
		t.Fatal("subdivided edges did not survive native reload", proof.Complete, err)
	}
	mass, err := got.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(mass.Volume-6) > 1e-10 {
		t.Fatal("subdivision changed physical volume", mass, err)
	}
	accepted := ExactStraightSewOptions{Sew: ExactSewOptions{MaxEdges: len(input.edges) + 1}, MaxPairTests: report.PairTests}
	if _, _, err := SewExactStraightSubdivisions(ctx, input.vertices, input.edges, input.faces, accepted); err != nil {
		t.Fatal("exact final edge and pair budgets should suffice", err)
	}
	for _, o := range []ExactStraightSewOptions{
		{Sew: ExactSewOptions{MaxEdges: len(input.edges)}}, {MaxPairTests: report.PairTests - 1},
	} {
		failed, partial, err := SewExactStraightSubdivisions(ctx, input.vertices, input.edges, input.faces, o)
		if !errors.Is(err, geom.ErrConvergence) || len(failed.faces) != 0 || len(partial.Segments) != 0 {
			t.Fatal("limited subdivision returned a partial result", failed.Topology(), partial, err)
		}
	}
}

func TestSewExactStraightSubdivisionsRejectGapAndCancel(t *testing.T) {
	input, _, smallA, _ := nonconformingStraightSewFixture(t)
	input.vertices[input.edges[smallA].End].X = math.Nextafter(input.vertices[input.edges[smallA].End].X, math.Inf(1))
	ctx := context.Background()
	failed, report, err := SewExactStraightSubdivisions(ctx, input.vertices, input.edges, input.faces, ExactStraightSewOptions{})
	if err == nil || len(failed.faces)+len(report.Vertices)+len(report.Segments) != 0 {
		t.Fatal("a single ULP mismatch was rounded into the result", failed.Topology(), report, err)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := SewExactStraightSubdivisions(ctx, input.vertices, input.edges, input.faces, ExactStraightSewOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled sewing", err)
	}
}
