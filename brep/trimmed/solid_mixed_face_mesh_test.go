package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestStoredMixedFaceLensPlacement(t *testing.T) {
	s := solidEllipticalLensFixture(t)
	faces := s.Faces()
	q, e := mixedStoredUseCurve(faces[0].Outer[0])
	if e != nil {
		t.Fatal(e)
	}
	h, e := mixedExactControls(q)
	if e != nil || len(h) != 3 {
		t.Fatal(e)
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
	transform := geom.Affine{{0, -1, 0, 2}, {1, 0, 0, -3}, {0, 0, 1, .5}}
	placed, e := s.Transformed(transform)
	if e != nil {
		t.Fatal(e)
	}
	o := SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .04}}}}
	for _, body := range []Solid{s, placed} {
		for fi := range body.faces {
			r, e := body.TessellateMixedFaceCertified(context.Background(), fi, o)
			if e != nil {
				t.Fatal(fi, e)
			}
			if r.ModelEdgeUsesCertified != len(body.faces[fi].Outer) || len(r.Coedges) != len(body.faces[fi].Outer) {
				t.Fatal("missing coedge proofs")
			}
			for _, coedge := range r.Coedges {
				u := body.faces[fi].Outer[coedge.Use]
				if coedge.Edge != u.Edge || coedge.FaceReversed != body.faces[fi].Reversed {
					t.Fatal("owner/orientation")
				}
				edge := body.edges[u.Edge]
				for j, id := range coedge.Vertices {
					q := new(big.Rat).Set(coedge.TrimParameters[j])
					if u.Reversed {
						q.Sub(big.NewRat(1, 1), q)
					}
					want := new(big.Rat).Add(ratInset(edge.Range.Min), new(big.Rat).Mul(new(big.Rat).Sub(ratInset(edge.Range.Max), ratInset(edge.Range.Min)), q))
					if want.Cmp(coedge.ModelParameters[j]) != 0 {
						t.Fatal("model range map")
					}
					f, _ := want.Float64()
					point, e := edge.Curve.Eval(f)
					if e != nil || point.Sub(r.Mesh.Vertices[id]).Length() > 1e-10 {
						t.Fatal("independent actual model evaluation", e)
					}
				}
			}
			for i, tri := range r.Mesh.Triangles {
				if r.CertifiedCurvedQuadraticTrimMesh.OriginalFace[i] != fi {
					t.Fatal("triangle face")
				}
				a, b, c := r.ExactUV[tri[0]], r.ExactUV[tri[1]], r.ExactUV[tri[2]]
				cross := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(c[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(c[0], a[0])))
				if (cross.Sign() < 0) != body.faces[fi].Reversed {
					t.Fatal("physical winding")
				}
			}
		}
	}
	wrong := s
	wrong.faces = s.Faces()
	wrong.faces[0].Outer[0].Edge = wrong.faces[0].Outer[1].Edge
	if denied, e := wrong.TessellateMixedFaceCertified(context.Background(), 0, o); e == nil || len(denied.Mesh.Vertices) != 0 {
		t.Fatal("wrong model edge")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if denied, e := s.TessellateMixedFaceCertified(ctx, 0, o); e != context.Canceled || len(denied.Mesh.Vertices) != 0 {
		t.Fatal("cancellation", e)
	}
	bad := s
	bad.faces = s.Faces()
	bad.faces[0].Outer[0].UVStart.X += .125
	if r, e := bad.TessellateMixedFaceCertified(context.Background(), 0, o); e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("mismatched stored corner")
	}
}
func TestStoredMixedStraightFace(t *testing.T) {
	s := testWarpedSplinePrism(t, false, false)
	o := SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .05}}}}
	r, e := s.TessellateMixedFaceCertified(context.Background(), 0, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Coedges) != 4 {
		t.Fatal("straight uses")
	}
	edges := s.Edges()
	id := s.faces[0].Outer[0].Edge
	model, ok := edges[id].Curve.(surfaceTrimCurve)
	if !ok {
		t.Fatal("fixture model type")
	}
	a, b := model.a, model.b
	model.a = geom.Vec2{X: a.X + (b.X-a.X)/4, Y: a.Y + (b.Y-a.Y)/4}
	model.b = geom.Vec2{X: model.a.X + (b.X-a.X)/8, Y: model.a.Y + (b.Y-a.Y)/8}
	edges[id].Curve = model
	edges[id].Range.Min = -2
	edges[id].Range.Max = 6
	changed, e := NewSolid(s.Vertices(), edges, s.Faces(), s.tolerance)
	if e != nil {
		t.Fatal("nonunit solid", e)
	}
	r, e = changed.TessellateMixedFaceCertified(context.Background(), 0, o)
	if e != nil {
		t.Fatal("nonunit adapter", e)
	}
	found := false
	for _, c := range r.Coedges {
		if c.Edge != id {
			continue
		}
		found = true
		for j, q := range c.ModelParameters {
			f, _ := q.Float64()
			point, e := edges[id].Curve.Eval(f)
			if e != nil || point.Sub(r.Mesh.Vertices[c.Vertices[j]]).Length() > 1e-12 {
				t.Fatal("nonunit witness", e)
			}
		}
	}
	if !found {
		t.Fatal("nonunit ancestry")
	}

}
