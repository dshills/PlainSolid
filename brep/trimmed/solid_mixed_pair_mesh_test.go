package trimmed

import (
	"context"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestStoredMixedPairedLensMismatch(t *testing.T) {
	slowtest.Skip(t)
	s := solidEllipticalLensFixture(t)
	edge := s.faces[0].Outer[0].Edge
	o := [2]SolidMixedFaceMeshOptions{{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .4}}}}, {Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .005}}}}}
	r, e := s.TessellateMixedFacePairCertified(context.Background(), [2]int{0, 1}, edge, o, 2)
	if e != nil {
		t.Fatal(e)
	}
	if r.RefinementRounds < 1 || len(r.SharedVertices) < 33 {
		t.Fatal("no mismatch refinement", r.RefinementRounds, len(r.SharedVertices))
	}
	counts := func(tris [][3]int, a, b int) int {
		n := 0
		for _, tri := range tris {
			for j, x := range tri {
				if x == a && tri[(j+1)%3] == b {
					n++
				}
			}
		}
		return n
	}
	for i, v := range r.SharedVertices {
		if r.Faces[0].Mesh.Vertices[v[0]] != r.Faces[1].Mesh.Vertices[v[1]] {
			t.Fatal("display identity")
		}
		f, _ := r.ModelParameters[i].Float64()
		point, e := s.edges[edge].Curve.Eval(f)
		if e != nil || point.Sub(r.Faces[0].Mesh.Vertices[v[0]]).Length() > 1e-10 {
			t.Fatal("original model witness")
		}
		if i > 0 {
			p := r.SharedVertices[i-1]
			af := counts(r.Faces[0].Mesh.Triangles, p[0], v[0])
			br := counts(r.Faces[1].Mesh.Triangles, v[1], p[1])
			if af != br {
				t.Fatal("opposite incidence")
			}
		}
	}
	wrong := s
	wrong.faces = s.Faces()
	wrong.faces[1].Reversed = !wrong.faces[1].Reversed
	if r, e := wrong.TessellateMixedFacePairCertified(context.Background(), [2]int{0, 1}, edge, o, 2); e == nil || len(r.SharedVertices) != 0 {
		t.Fatal("same effective orientation")
	}
	wrong = s
	wrong.faces = s.Faces()
	wrong.faces[0].Outer = append(wrong.faces[0].Outer, wrong.faces[0].Outer[0])
	if r, e := wrong.TessellateMixedFacePairCertified(context.Background(), [2]int{0, 1}, edge, o, 2); e == nil || len(r.SharedVertices) != 0 {
		t.Fatal("ambiguous occurrence")
	}
	for _, rounds := range []int{0, 9} {
		if r, e := s.TessellateMixedFacePairCertified(context.Background(), [2]int{0, 1}, edge, o, rounds); e == nil || len(r.SharedVertices) != 0 {
			t.Fatal("round limit")
		}
	}
}
