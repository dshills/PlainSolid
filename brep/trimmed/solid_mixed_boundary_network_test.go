package trimmed

import (
	"context"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestStoredMixedCompleteBoundaryNetwork(t *testing.T) {
	slowtest.Skip(t)
	for _, lens := range []bool{true, false} {
		s := solidEllipticalLensFixture(t)
		if !lens {
			s = testWarpedSplinePrism(t, false, false)
		}
		if lens {
			faces := s.Faces()
			curve, e := mixedStoredUseCurve(faces[0].Outer[0])
			if e != nil {
				t.Fatal(e)
			}
			h, e := mixedExactControls(curve)
			if e != nil || len(h) != 3 {
				t.Fatal("quadratic source", e)
			}
			elevated := make([][3]*big.Rat, 4)
			for i := range elevated {
				for k := 0; k < 3; k++ {
					switch i {
					case 0:
						elevated[i][k] = new(big.Rat).Set(h[0][k])
					case 3:
						elevated[i][k] = new(big.Rat).Set(h[2][k])
					default:
						elevated[i][k] = new(big.Rat).Add(new(big.Rat).Mul(h[i-1][k], big.NewRat(int64(i), 3)), new(big.Rat).Mul(h[i][k], big.NewRat(int64(3-i), 3)))
					}
				}
			}
			faces[0].Outer[0].Curve, e = NewExactBezierUV(elevated)
			if e != nil {
				t.Fatal(e)
			}
			s, e = NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance)
			if e != nil {
				t.Fatal(e)
			}
		}
		options := make([]SolidMixedFaceMeshOptions, len(s.faces))
		for i := range options {
			deflection := .05
			if lens {
				deflection = .4
				if i == 1 {
					deflection = .005
				}
			}
			options[i] = SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: deflection}}}}
		}
		r, e := s.TessellateMixedBoundaryNetworkCertified(context.Background(), options, 2)
		if e != nil {
			t.Fatal(lens, e)
		}
		if len(r.EdgeVertices) != len(s.edges) || len(r.CornerVertices) != len(s.vertices) || len(r.Faces) != len(s.faces) {
			t.Fatal("incomplete original ownership")
		}
		if lens && r.RefinementRounds < 1 {
			t.Fatal("no global mismatch refinement")
		}
		for edge, ids := range r.EdgeVertices {
			if ids[0] != r.CornerVertices[s.edges[edge].Start] || ids[len(ids)-1] != r.CornerVertices[s.edges[edge].End] {
				t.Fatal("corner identity")
			}
			for j, id := range ids {
				f, _ := r.EdgeParameters[edge][j].Float64()
				point, e := s.edges[edge].Curve.Eval(f)
				if e != nil || point.Sub(r.Points[id]).Length() > 1e-10 {
					t.Fatal("model witness")
				}
			}
		}
		for fi, face := range r.Faces {
			for _, loop := range face.BoundaryLoops {
				for _, local := range loop {
					id, ok := r.FaceVertices[fi][local]
					if !ok || face.Mesh.Vertices[local] != r.Points[id] {
						t.Fatal("boundary not completely shared")
					}
				}
			}
		}
		bad := s
		bad.faces = s.Faces()
		bad.faces[0].Reversed = !bad.faces[0].Reversed
		if r, e := bad.TessellateMixedBoundaryNetworkCertified(context.Background(), options, 2); e == nil || len(r.Points) != 0 {
			t.Fatal("orientation failure")
		}
	}
}
