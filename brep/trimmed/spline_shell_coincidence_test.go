package trimmed

import (
	"bytes"
	"context"
	"errors"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"testing"
)

func TestCoincidentSplineShellRefinedReordered(t *testing.T) {
	ctx := context.Background()
	a := testWarpedSplinePrism(t, false, false)
	edge := a.edges[0]
	b, split, e := a.SplitSplineEdgeExact(ctx, 0, (edge.Range.Min+edge.Range.Max)/2, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	for i, j := 0, len(b.faces)-1; i < j; i, j = i+1, j-1 {
		b.faces[i], b.faces[j] = b.faces[j], b.faces[i]
	}
	var stream bytes.Buffer
	if e = WriteJSON(&stream, b); e != nil {
		t.Fatal(e)
	}
	b, e = ReadJSONContext(ctx, &stream, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	// The old full-rectangle proof cannot admit this original subdivided use.
	if p, e := b.ValidateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{}); e == nil && p.Complete {
		t.Fatal("fixture lost distinct scope")
	}
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
		out, h, e := CombineCoincidentSplineShells(ctx, a, b, op, SplineShellCoincidenceOptions{})
		if e != nil || !h.Complete || len(h.Faces) != len(a.faces) {
			t.Fatal(op, h, e)
		}
		if h.VerticesB[split.Vertex].VertexA != -1 || h.VerticesB[split.Vertex].EdgeA != 0 || h.VerticesB[split.Vertex].Parameter.String() != "1/2" {
			t.Fatal("lost split vertex", h.VerticesB[split.Vertex])
		}
		if h.EdgesB[split.First].EdgeA != 0 || h.EdgesB[split.Second].EdgeA != 0 {
			t.Fatal("lost source edge")
		}
		if op == parametric.Difference {
			if len(out.faces) != 0 {
				t.Fatal("nonempty difference")
			}
			for _, row := range h.FacesB {
				if len(row) != 0 {
					t.Fatal("false retained output")
				}
			}
		} else {
			var x, y bytes.Buffer
			_ = WriteJSON(&x, a)
			_ = WriteJSON(&y, out)
			if !bytes.Equal(x.Bytes(), y.Bytes()) {
				t.Fatal("changed original topology")
			}
			for i, row := range h.FacesB {
				if len(row) != 1 || row[0] != len(a.faces)-1-i {
					t.Fatal("wrong original parent", h.FacesB)
				}
			}
		}
	}
}
func TestCoincidentSplineShellRejects(t *testing.T) {
	ctx := context.Background()
	a := testWarpedSplinePrism(t, false, false)
	for _, change := range []func(*Solid){func(b *Solid) { b.faces = b.faces[:len(b.faces)-1] }, func(b *Solid) { b.faces[0].Reversed = !b.faces[0].Reversed }, func(b *Solid) { b.vertices[0].X += 1e-12 }} {
		b := Solid{vertices: a.Vertices(), edges: a.Edges(), faces: a.Faces(), tolerance: a.tolerance}
		change(&b)
		out, h, e := CombineCoincidentSplineShells(ctx, a, b, parametric.Union, SplineShellCoincidenceOptions{})
		if e == nil || h.Complete || len(out.faces) != 0 || len(h.Faces) > 0 {
			t.Fatal("non-atomic false equality", h, e)
		}
	}
	b := testWarpedSplinePrism(t, false, false)
	for i, j := 0, len(b.faces)-1; i < j; i, j = i+1, j-1 {
		b.faces[i], b.faces[j] = b.faces[j], b.faces[i]
	}
	if out, h, e := CombineCoincidentSplineShells(ctx, a, b, parametric.Intersection, SplineShellCoincidenceOptions{MaxPairs: 1}); e == nil || h.Complete || len(out.faces) > 0 {
		t.Fatal("pair budget")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, h, e := CombineCoincidentSplineShells(canceled, a, b, parametric.Union, SplineShellCoincidenceOptions{}); !errors.Is(e, context.Canceled) || h.Complete {
		t.Fatal(h, e)
	}
}

func TestCoincidentSplineShellRejectsDeceptiveInterior(t *testing.T) {
	a := testWarpedSplinePrism(t, false, false)
	b := testWarpedSplinePrism(t, false, true) // same boundary poles, altered roof interior
	out, h, e := CombineCoincidentSplineShells(context.Background(), a, b, parametric.Intersection, SplineShellCoincidenceOptions{})
	if e == nil || h.Complete || len(out.faces) != 0 || len(h.Faces) != 0 {
		t.Fatal("boundary-only coincidence", h, e)
	}
}
