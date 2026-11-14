package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
	"testing"
)

// Exact degree-three representation of (x,y,z+x³/8+y/4) on each cube face.
// Interior homogeneous coordinates include thirds/twelfths, while original
// corner geometry stays exactly dyadic. Old degree-four edge models remain an
// independently authored representation of the same full boundary curves.
func exactCubicStoredStock(t *testing.T) Solid {
	t.Helper()
	ctx := context.Background()
	s, _ := polynomialClipStock(t)
	faces := s.Faces()
	for axis := 0; axis < 3; axis++ {
		u, v := (axis+1)%3, (axis+2)%3
		if axis == 1 {
			u, v = 0, 2
		}
		for side := 0; side < 2; side++ {
			var h [][4]*big.Rat
			for i := 0; i < 4; i++ {
				for j := 0; j < 4; j++ {
					p := [4]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}
					p[axis].SetInt64(int64(side))
					p[u].SetFrac64(int64(i), 3)
					p[v].SetFrac64(int64(j), 3)
					cube := new(big.Rat)
					if u == 0 {
						if i == 3 {
							cube.SetInt64(1)
						}
					} else if v == 0 {
						if j == 3 {
							cube.SetInt64(1)
						}
					} else if side == 1 {
						cube.SetInt64(1)
					}
					p[2].Add(p[2], new(big.Rat).Quo(cube, big.NewRat(8, 1)))
					p[2].Add(p[2], new(big.Rat).Quo(p[1], big.NewRat(4, 1)))
					h = append(h, p)
				}
			}
			patch, e := geom.NewExactHomogeneousBezierSurface(ctx, 3, 3, h, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			adapter, e := geom.NewExactBezierSurface(patch)
			if e != nil {
				t.Fatal(e)
			}
			faces[2*axis+side].Surface = adapter
		}
	}
	out, e := NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	return out
}

func TestStoredExactCubicFaceCertified(t *testing.T) {
	ctx := context.Background()
	s := exactCubicStoredStock(t)
	o := SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Span: MultiSpanTriangleBoundOptions{MaxDegree: 3}, Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .1}}}}
	if r, e := s.TessellateMixedFaceCertified(ctx, 5, SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Face: o.Face.Face}}); e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("default degree gate widened")
	}
	o.Face.Face.RectangularGrid = true
	o.Face.Face.Cuts = make([][][]*big.Rat, 1)
	o.Face.Face.Cuts[0] = make([][]*big.Rat, 4)
	for i := range o.Face.Face.Cuts[0] {
		for j := 0; j <= 4; j++ {
			o.Face.Face.Cuts[0][i] = append(o.Face.Face.Cuts[0][i], big.NewRat(int64(j), 4))
		}
	}
	r, e := s.TessellateMixedFaceCertified(ctx, 5, o)
	if e != nil {
		t.Fatal(e)
	}
	if r.ModelEdgeUsesCertified != 4 {
		t.Fatal("model correspondence lost")
	}
	checkCubicMeshOracle(t, r.CertifiedMixedExactCurvedMesh, 1)
	var stream bytes.Buffer
	if e = WriteJSON(&stream, s); e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadJSONContext(ctx, &stream, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := loaded.TessellateMixedFaceCertified(ctx, 5, o)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(r.Mesh, q.Mesh) || r.FillSquaredDistance.Cmp(q.FillSquaredDistance) != 0 {
		t.Fatal("native source/reload changed")
	}
	low := o
	low.Face.MaxResidualOperations = 1
	if r, e := s.TessellateMixedFaceCertified(ctx, 5, low); e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("aggregate arithmetic limit", e)
	}
}

func checkCubicMeshOracle(t *testing.T, r CertifiedMixedExactCurvedMesh, offset int64) {
	t.Helper()
	for _, tri := range r.Mesh.Triangles {
		u, v := new(big.Rat), new(big.Rat)
		var display [3]*big.Rat
		for k := range display {
			display[k] = new(big.Rat)
		}
		for _, id := range tri {
			u.Add(u, r.ExactUV[id][0])
			v.Add(v, r.ExactUV[id][1])
			p := exactBoundPoint(r.Mesh.Vertices[id])
			for k := range display {
				display[k].Add(display[k], p[k])
			}
		}
		u.Quo(u, big.NewRat(3, 1))
		v.Quo(v, big.NewRat(3, 1))
		for k := range display {
			display[k].Quo(display[k], big.NewRat(3, 1))
		}
		z := new(big.Rat).Mul(u, u)
		z.Mul(z, u)
		z.Quo(z, big.NewRat(8, 1))
		z.Add(z, new(big.Rat).Quo(v, big.NewRat(4, 1)))
		z.Add(z, big.NewRat(offset, 1))
		diff := ddSub([3]*big.Rat{u, v, z}, display)
		if ddDot(diff, diff).Cmp(r.FillSquaredDistance) > 0 {
			t.Fatal("independent cubic interior witness exceeds bound")
		}
	}
}

func TestExactCubicMixedCurvedHole(t *testing.T) {
	ctx := context.Background()
	s := exactCubicStoredStock(t)
	support := s.faces[5].Surface.(*geom.ExactBezierSurface)
	outer, hole := mixedAffineLoop(t, big.NewRat(3, 8), false), mixedAffineLoop(t, big.NewRat(1, 8), true)
	o := MixedExactMultiSpanMeshOptions{Span: MultiSpanTriangleBoundOptions{MaxDegree: 3}, Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .05}}}
	r, e := TessellateMixedExactMultiSpanRegionCertified(ctx, support.ExactPatch(), outer, [][]PCurve{hole}, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.BoundaryLoops) != 2 || r.FillSquaredDistance.Cmp(big.NewRat(1, 400)) > 0 {
		t.Fatal("missing holed proof")
	}
	checkCubicMeshOracle(t, r, 1)
	for id, uv := range r.ExactUV {
		z := new(big.Rat).Mul(uv[0], uv[0])
		z.Mul(z, uv[0])
		z.Quo(z, big.NewRat(8, 1))
		z.Add(z, new(big.Rat).Quo(uv[1], big.NewRat(4, 1)))
		z.Add(z, big.NewRat(1, 1))
		if r.ExactPoints[id][2].Cmp(z) != 0 {
			t.Fatal("nonbinary cubic model changed")
		}
	}
	refined, e := support.ExactPatch().InsertUKnot(.5, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if q, e := TessellateMixedExactMultiSpanRegionCertified(ctx, refined, outer, [][]PCurve{hole}, o); e != nil || len(q.Mesh.Vertices) == 0 {
		t.Fatal("exact removable knot rejected", e)
	}
	low := o
	low.MaxResidualOperations = 1
	if q, e := TessellateMixedExactMultiSpanRegionCertified(ctx, support.ExactPatch(), outer, [][]PCurve{hole}, low); e == nil || len(q.Mesh.Vertices) != 0 {
		t.Fatal("partial residual budget")
	}
	low = o
	low.Span.MaxDegree = 9
	if _, e := TessellateMixedExactMultiSpanRegionCertified(ctx, support.ExactPatch(), outer, [][]PCurve{hole}, low); e == nil {
		t.Fatal("unsupported degree")
	}
}
