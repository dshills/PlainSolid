package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedPolygonExactSupportConcaveHole(t *testing.T) {
	a, _ := concaveOriginalHoleFixture(t)
	var h [][4]*big.Rat
	for i := int64(0); i < 2; i++ {
		for j := int64(0); j < 2; j++ {
			h = append(h, [4]*big.Rat{big.NewRat(i, 1), big.NewRat(j, 1), big.NewRat(i*j, 3), big.NewRat(1, 1)})
		}
	}
	p, e := geom.NewExactHomogeneousBezierSurface(context.Background(), 1, 1, h, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := p.ToBSplineSurface(); e == nil {
		t.Fatal("fixture must require nonbinary support")
	}
	surface, e := geom.NewExactBezierSurface(p)
	if e != nil {
		t.Fatal(e)
	}
	a.faces[0].Surface = surface
	for _, reversed := range []bool{false, true} {
		a.faces[0].Reversed = reversed
		r, e := a.TessellatePolygonFaceCertified(context.Background(), 0, CertifiedPatchMeshOptions{Deflection: .02})
		if e != nil {
			t.Fatal(e)
		}
		if !r.BothDirections || len(r.Uses) != 10 || len(r.Loops) != 2 || r.OriginalFace != 0 {
			t.Fatal("missing certificate or lineage")
		}
		area := 0.
		for i, tri := range r.Mesh.Triangles {
			u := r.UVTriangles[i]
			signed := ((u[1].X-u[0].X)*(u[2].Y-u[0].Y) - (u[1].Y-u[0].Y)*(u[2].X-u[0].X)) / 2
			if (signed < 0) != reversed || signed == 0 {
				t.Fatal("projected orientation")
			}
			area += math.Abs(signed)
			// Independent exact graph sagitta at triangle barycenter: z=uv/3.
			x, y, z := 0., 0., 0.
			for _, id := range tri {
				q := r.Mesh.Vertices[id]
				x += q.X / 3
				y += q.Y / 3
				z += q.Z / 3
			}
			errorZ := math.Abs(z - x*y/3)
			bound, _ := r.SquaredErrors[i].Float64()
			if errorZ*errorZ > bound+1e-28 || bound > .02*.02 {
				t.Fatal("independent graph error", errorZ, bound)
			}
		}
		if math.Abs(area-15./32) > 1e-14 {
			t.Fatal("trim area", area)
		}
		for _, use := range r.Uses {
			if use.FaceReversed != reversed || use.Parameters[0].Sign() != 0 || use.Parameters[len(use.Parameters)-1].Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal("original traversal")
			}
			loop := a.faces[0].Outer
			if use.Loop == 1 {
				loop = a.faces[0].Holes[0]
			}
			original := loop[use.Use]
			if original.Edge != use.Edge {
				t.Fatal("source identity")
			}
			for i, id := range use.Vertices {
				uv := r.VertexUV[id]
				for k, ends := range [][2]float64{{original.UVStart.X, original.UVEnd.X}, {original.UVStart.Y, original.UVEnd.Y}} {
					v := new(big.Rat).Add(ratInset(ends[0]), new(big.Rat).Mul(use.Parameters[i], new(big.Rat).Sub(ratInset(ends[1]), ratInset(ends[0]))))
					if v.Cmp(ratInset([]float64{uv.X, uv.Y}[k])) != 0 {
						t.Fatal("UV lineage")
					}
				}
			}
		}
	}
	tensor, e := geom.NewExactTensorSurface(context.Background(), p, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a.faces[0].Surface = tensor
	if r, e := a.TessellatePolygonFaceCertified(context.Background(), 0, CertifiedPatchMeshOptions{Deflection: .02}); e != nil || !r.BothDirections || len(r.Uses) != 10 {
		t.Fatal("exact tensor support", e)
	}
	if r, e := a.TessellatePolygonFaceCertified(context.Background(), 0, CertifiedPatchMeshOptions{Deflection: .000001, MaxTriangleChecks: 1}); e == nil || len(r.Mesh.Vertices) > 0 || len(r.Uses) > 0 {
		t.Fatal("atomic mesh quota")
	}
	a.faces[0].Holes[0][1].UVEnd = geom.Vec2{X: .5, Y: .5}
	a.faces[0].Holes[0][2].UVStart = geom.Vec2{X: .5, Y: .5}
	if r, e := a.TessellatePolygonFaceCertified(context.Background(), 0, CertifiedPatchMeshOptions{Deflection: .02}); e == nil || len(r.Mesh.Vertices) > 0 {
		t.Fatal("touching hole accepted")
	}
}
