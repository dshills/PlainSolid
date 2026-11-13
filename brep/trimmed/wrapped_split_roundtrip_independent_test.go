package trimmed

import (
	"bytes"
	"context"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func TestIndependentWrappedRepeatedSplitRoundTripMesh(t *testing.T) {
	ctx := context.Background()
	f, e := WrappedSplineFace(ctx, independentWrappedToroid(t), false, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := Sew(ctx, []SewFace{f}, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	s, first, e := s.SplitSplineEdgeExact(ctx, 0, .5, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	s, _, e = s.SplitSplineEdgeExact(ctx, first.First, .25, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	for pass := 0; pass < 2; pass++ {
		var b bytes.Buffer
		if e := WriteJSON(&b, s); e != nil {
			t.Fatal(e)
		}
		s, e = ReadJSONContext(ctx, &b, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		top := s.Topology()
		if top.Vertices != 3 || top.Edges != 4 || top.Faces != 1 || top.EulerCharacteristic != 0 || len(top.Shells) != 1 {
			t.Fatal(top)
		}
		proof, e := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !proof.Complete {
			t.Fatal(proof, e)
		}
		counts := map[int]int{}
		for _, u := range s.faces[0].Outer {
			counts[u.Edge]++
		}
		for i := range s.edges {
			if counts[i] != 2 {
				t.Fatal("split seam multiplicity", counts)
			}
		}
		m, e := s.refine(ctx, 4, 0, 0)
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal("restored repeated seam closed mesh", e)
		}
		if m.SignedVolume() < 9 || m.SignedVolume() > 11 {
			t.Fatal("restored mesh material", m.SignedVolume())
		}
	}
}
