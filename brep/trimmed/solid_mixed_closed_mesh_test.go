package trimmed

import (
	"context"
	"math"
	"plainsolid/collision"
	"testing"
)

func TestStoredMixedClosedDisplays(t *testing.T) {
	for _, lens := range []bool{true, false} {
		s := solidEllipticalLensFixture(t)
		if !lens {
			s = testWarpedSplinePrism(t, false, false)
		}
		o := StoredMixedClosedMeshOptions{Faces: make([]SolidMixedFaceMeshOptions, len(s.faces))}
		for fi := range o.Faces {
			d := .03
			if lens {
				d = .02
			}
			o.Faces[fi] = SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: d}}}}
		}
		r, e := s.TessellateMixedClosedCertified(context.Background(), o)
		if e != nil {
			t.Fatal(lens, e)
		}
		if e = r.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if !r.DisplayCollision.Complete || len(r.DisplayCollision.Contacts) != 0 || r.DisplayVolume.Sign() <= 0 {
			t.Fatal("missing embedding")
		}
		edges := map[[2]int]bool{}
		for _, tri := range r.Mesh.Triangles {
			for j, a := range tri {
				b := tri[(j+1)%3]
				if a > b {
					a, b = b, a
				}
				edges[[2]int{a, b}] = true
			}
		}
		if len(r.Mesh.Vertices)-len(edges)+len(r.Mesh.Triangles) != 2 {
			t.Fatal("independent Euler sphere")
		}
		expected, tolerance := 5., .05
		if lens {
			expected = math.Pi / 1024
			tolerance = math.Pi / 1024 * .15
		}
		v, _ := r.DisplayVolume.Float64()
		if math.Abs(v-expected) > tolerance {
			t.Fatal("independent analytic volume", lens, v, expected)
		}
		t.Logf("lens=%v vertices=%d triangles=%d exactDisplayVolume=%.12g analytic=%.12g pairs=%d", lens, len(r.Mesh.Vertices), len(r.Mesh.Triangles), v, expected, r.DisplayCollision.BroadPhasePairs)
		for fi, ids := range r.FaceVertices {
			for local, global := range ids {
				if r.Network.Faces[fi].Mesh.Vertices[local] != r.Mesh.Vertices[global] {
					t.Fatal("global ownership")
				}
			}
		}
		limited := o
		limited.MaxTriangles = 1
		if r, e := s.TessellateMixedClosedCertified(context.Background(), limited); e == nil || len(r.Mesh.Vertices) != 0 {
			t.Fatal("aggregate amount")
		}
		limited = o
		limited.Collision = collision.Options{MaxPairs: 1}
		if r, e := s.TessellateMixedClosedCertified(context.Background(), limited); e == nil || len(r.Mesh.Vertices) != 0 {
			t.Fatal("contact budget")
		}
	}
}
