//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestUnifySameDomainCurvedSplineRectangles(t *testing.T) {
	ctx := context.Background()
	s, f := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	face := 5
	left, right := f[face].domain, f[face].domain
	left.UMax, right.UMin = .5, .5
	top := s.faces[face]
	// Split both incident model edges so the original surrounding faces
	// share the topology of the two independently trimmed roof patches.
	for _, edge := range []int{top.Outer[0].Edge, top.Outer[2].Edge} {
		var e error
		s, _, e = s.SplitSplineEdgeExact(ctx, edge, .5, edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
	}
	a, e := SplineFaceRectangle(f[face].surface, left, top.Reversed)
	if e != nil {
		t.Fatal(e)
	}
	// Independently encode the second chart after knot refinement. Its
	// pointer and basis differ, while its original UV map is exact.
	patch, e := geom.NewSurfacePatch(f[face].surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	refined, e := patch.InsertUKnot(.75, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	other, e := refined.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	b, e := SplineFaceRectangle(other, right, top.Reversed)
	if e != nil {
		t.Fatal(e)
	}
	parts := localSewFaces(s)
	parts[face] = a
	parts = append(parts, b)
	split, _, e := Sew(ctx, parts, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	out, h, e := split.UnifyAdjacentSplineFaces(ctx, face, len(parts)-1, UnifySplineFacesOptions{})
	if e != nil || len(h.RemovedEdges) != 1 || h.Edges[h.RemovedEdges[0]] != -1 ||
		h.Faces[face] != h.Faces[len(parts)-1] || len(out.faces) != 6 ||
		out.faces[h.Faces[face]].Surface != f[face].surface || len(out.faces[h.Faces[face]].Outer) != 6 {
		t.Fatal(h, out.Topology(), e)
	}
	p, e := out.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-1) > 1e-10 {
		t.Fatal(p, e)
	}
	if _, _, e := out.UnifyAdjacentSplineFaces(ctx, 0, 1, UnifySplineFacesOptions{}); e == nil {
		t.Fatal("merged different supports")
	}
	// Each surviving subdivision is a valence-two point. Merge back to the
	// original edge count and verify that the rectangle proof accepts it.
	for len(out.edges) > 12 {
		changed := false
		for i := range out.edges {
			for j := range out.edges {
				next, _, e := out.MergeSplitSplineEdgeExact(ctx, i, j, edgecheck.Options{})
				if e == nil {
					out, changed = next, true
					break
				}
			}
			if changed {
				break
			}
		}
		if !changed {
			t.Fatal("could not remove surviving valence-two subdivision", out.Topology())
		}
	}
	pf, e := out.ValidateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{})
	if e != nil || !pf.Complete {
		t.Fatal(pf, e)
	}
}

func TestMergeWrappedSeamAfterNativeReload(t *testing.T) {
	ctx := context.Background()
	s, _, e := NewWrappedToroid(ctx, independentWrappedToroid(t), 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	split, cut, e := s.SplitSplineEdgeExact(ctx, 0, .5, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, split); e != nil {
		t.Fatal(e)
	}
	split, e = ReadJSON(&data, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	out, h, e := split.MergeSplitSplineEdgeExact(ctx, cut.First, cut.Second, edgecheck.Options{})
	if e != nil || h.Edges[cut.First] != h.Edges[cut.Second] || h.Vertices[cut.Vertex] != -1 || out.Topology().Edges != 2 {
		t.Fatal(h, out.Topology(), e)
	}
}
