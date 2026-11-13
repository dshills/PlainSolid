package trimmed

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestSplitFacePath(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 3, Y: 2, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	f := s.Faces()[0]
	a, c := f.Outer[0].UVStart, f.Outer[2].UVStart
	mid := mix(a, c, .5)
	mid.Y += .12
	path := []CutSegment{{a, mid, QuadraticUV{a, mix(a, mid, .5), mid}}, {mid, c, nil}}
	ctx := context.Background()
	r, h, e := s.SplitFacePath(ctx, 0, 0, 2, path)
	if e != nil || !h.UVTopologyCertified || len(h.Edges) != 2 || len(h.InteriorVertices) != 1 || len(r.Faces()) != 7 {
		t.Fatal(h, e)
	}
	var raw bytes.Buffer
	if e = WriteJSON(&raw, r); e != nil {
		t.Fatal(e)
	}
	r, e = ReadJSON(&raw, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := r.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-24) > 1e-10 || math.Abs(p.Area-52) > 1e-10 {
		t.Fatal(p, e)
	}
	out, e := r.Tessellate(ctx, parametric.TessellationOptions{Deflection: .01})
	if e != nil || out.ValidateSolid() != nil {
		t.Fatal(e)
	}
	path[1].Start = a
	if _, _, e = s.SplitFacePath(ctx, 0, 0, 2, path); e == nil {
		t.Fatal("accepted disconnected path")
	}
	path = []CutSegment{{a, f.Outer[1].UVStart, nil}, {f.Outer[1].UVStart, c, nil}}
	if _, _, e = s.SplitFacePath(ctx, 0, 0, 2, path); e == nil {
		t.Fatal("accepted a cut retracing the boundary")
	}
}

func TestSplitFaceAcrossSharedEdges(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 3, Y: 2, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	f := s.Faces()[0]
	edge := func(i int) EdgeAnchor {
		id := f.Outer[i].Edge
		return EdgeAnchor{id, s.Edges()[id].Range.Max / 2}
	}
	ctx := context.Background()
	r, h, e := s.SplitFaceAcrossEdges(ctx, 0, edge(0), edge(2), func(a, b geom.Vec2) ([]CutSegment, error) {
		c := mix(a, b, .5)
		c.X += .2
		return []CutSegment{{a, b, QuadraticUV{a, c, b}}}, nil
	})
	if e != nil || h.Path.First != 0 || h.Path.Second != 6 || !h.Path.UVTopologyCertified ||
		h.Start.Vertex != 8 || h.End.Vertex != 9 || len(r.Edges()) != 15 {
		t.Fatal(h, e)
	}
	m, e := r.Tessellate(ctx, parametric.TessellationOptions{Deflection: .01})
	if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-24) > 1e-9 {
		t.Fatal(m.SignedVolume(), e)
	}
}
