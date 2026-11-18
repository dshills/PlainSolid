package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCertifiedConicalFilletBand(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 8, 6, o)
	if e != nil {
		t.Fatal(e)
	}
	w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Radius: 1}}, o)
	if e != nil {
		t.Fatal(e)
	}
	for end := 0; end < 2; end++ {
		m, e := q.TessellateFilletBandCertified(ctx, end, CertifiedPatchMeshOptions{Deflection: .1, MaxSubdivisions: 16, MaxTriangleChecks: 12000})
		if e != nil {
			t.Fatal(end, e)
		}
		t.Log(end, len(m.Mesh.Triangles), m.Subdivisions, m.TriangleChecks)
		if !m.BothDirections || m.SourceFace != q.fillets.Blends[end] || len(m.SquaredErrors) != len(m.Mesh.Triangles) {
			t.Fatal("source certificate")
		}
		counts := map[[2]int]int{}
		key := func(a, b int) [2]int {
			if a > b {
				a, b = b, a
			}
			return [2]int{a, b}
		}
		for i, tri := range m.Mesh.Triangles {
			if m.SquaredErrors[i].Cmp(new(big.Rat).Mul(new(big.Rat).SetFloat64(.1), new(big.Rat).SetFloat64(.1))) > 0 {
				t.Fatal("error")
			}
			for k := 0; k < 3; k++ {
				counts[key(tri[k], tri[(k+1)%3])]++
			}
			uv := m.UVTriangles[i]
			u := (uv[0].X + uv[1].X + uv[2].X) / 3
			v := (uv[0].Y + uv[1].Y + uv[2].Y) / 3
			w := q.solid.faces[m.SourceFace].Surface.(*geom.WrappedSurfacePatch)
			sample, e := w.Eval(u, v)
			if e != nil {
				t.Fatal(e)
			}
			linear := m.Mesh.Vertices[tri[0]].Add(m.Mesh.Vertices[tri[1]]).Add(m.Mesh.Vertices[tri[2]]).Scale(1.0 / 3)
			if sample.Sub(linear).Length() > .1 {
				t.Fatal("independent barycentric witness")
			}
		}
		boundary := map[[2]int]bool{}
		for role, row := range m.Rims {
			if len(row) != len(m.UParameters)-1 {
				t.Fatal("rim count")
			}
			for _, segment := range row {
				pair := key(segment.Vertices[0], segment.Vertices[1])
				boundary[pair] = true
				if counts[pair] != 1 {
					t.Fatal("rim incidence")
				}
				for k, id := range segment.Vertices {
					parameter, _ := segment.Parameters[k].Float64()
					point, e := q.solid.edges[segment.Edge].Curve.Eval(parameter)
					if e != nil || point.Sub(m.Mesh.Vertices[id]).Length() > 1e-12 {
						t.Fatal("original edge ancestry", e)
					}
					c := q.fillets.Circles[end]
					radius, height := c.Center[0], q.z[end]
					if role == 1 {
						radius, height = c.Tangency[0], c.Tangency[1]
					}
					r, _ := radius.Float64()
					z, _ := height.Float64()
					p := m.Mesh.Vertices[id]
					if math.Abs(math.Hypot(p.X, p.Y)-r) > 1e-12 || p.Z != z {
						t.Fatal("analytic rim")
					}
				}
			}
		}
		for pair, count := range counts {
			if boundary[pair] {
				continue
			}
			if count != 2 {
				t.Fatal("interior or periodic seam crack", pair, count)
			}
		}
		for j := 1; j < len(m.SeamVertices); j++ {
			if counts[key(m.SeamVertices[j-1], m.SeamVertices[j])] != 2 {
				t.Fatal("periodic seam")
			}
		}
		bad, e := q.TessellateFilletBandCertified(ctx, end, CertifiedPatchMeshOptions{Deflection: .1, MaxTriangleChecks: 1})
		if e == nil || len(bad.Mesh.Vertices) != 0 || bad.Rims[0] != nil {
			t.Fatal("atomic budget")
		}

	}
}
