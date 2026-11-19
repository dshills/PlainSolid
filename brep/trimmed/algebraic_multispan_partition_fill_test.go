//go:build research

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func multispanPartitionFixture(t *testing.T, crease bool) (AlgebraicSubdivision, [3]*big.Rat) {
	source := solidEllipticalLensFixture(t)
	faces, edges, vertices := source.Faces(), source.Edges(), source.Vertices()
	mapping := map[geom.Surface]geom.Surface{}
	for i, f := range faces {
		p, e := exactFaceSupportPatch(f, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = p.InsertUKnot(.5, 2, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = p.InsertVKnot(.5, 2, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		surface, e := p.ToBSplineSurface()
		if e != nil {
			t.Fatal(e)
		}
		if crease {
			data := surface.Data()
			for j, p := range data.Poles {
				data.Poles[j].Z += math.Abs(p.X-.5) / 8
			}
			surface, e = geom.NewBSplineSurface(data.DegreeU, data.DegreeV, data.CountU, data.CountV, data.Poles, data.KnotsU, data.KnotsV, data.Weights)
			if e != nil {
				t.Fatal(e)
			}
		}
		mapping[f.Surface] = surface
		faces[i].Surface = surface
	}
	for i, edge := range edges {
		model := edge.Curve.(surfaceTrimCurve)
		model.surface = mapping[model.surface]
		edges[i].Curve = model
	}
	if crease {
		for i, p := range vertices {
			vertices[i].Z += math.Abs(p.X-.5) / 8
		}
	}
	source, e := NewSolid(vertices, edges, faces, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	return algebraicPartitionForSolid(t, source)
}
func TestMultiSpanCurvedAlgebraicPartitionFill(t *testing.T) {
	slowtest.Skip(t)
	for _, crease := range []bool{false, true} {
		sub, line := multispanPartitionFixture(t, crease)
		o := MultiSpanAlgebraicPartitionMeshOptions{Partition: CurvedAlgebraicPartitionMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .02}}}
		got, e := sub.TessellateMultiSpanCurvedPartitionCertified(context.Background(), 0, line, o)
		if e != nil {
			t.Fatal("crease", crease, e)
		}
		if got.SpanCellChecks < 4 || got.SpanPieceChecks <= len(got.Mesh.Triangles) || got.FillSquaredDistance.Cmp(big.NewRat(1, 2500)) > 0 {
			t.Fatal("knot proof accounting", got.SpanCellChecks, got.SpanPieceChecks)
		}
		volume := 0.
		crossings := 0
		for i, tri := range got.Mesh.Triangles {
			u, v, z := 0., 0., 0.
			minU, maxU := 1., 0.
			gap := 0.
			for _, id := range tri {
				x, _ := got.ExactUV[id][0].Float64()
				y, _ := got.ExactUV[id][1].Float64()
				u += x / 3
				v += y / 3
				z += got.Mesh.Vertices[id].Z / 3
				if x < minU {
					minU = x
				}
				if x > maxU {
					maxU = x
				}
				base := 2.
				if crease {
					base += math.Abs(x-.5) / 8
				}
				gap += (got.Mesh.Vertices[id].Z - base) / 3
			}
			if minU < .5 && maxU > .5 {
				crossings++
			}
			want := 2 + 1./16 - (u-.5)*(u-.5) - 4*(v-.5)*(v-.5)
			if crease {
				want += math.Abs(u-.5) / 8
			}
			bound, _ := got.SquaredErrors[i].Float64()
			if (z-want)*(z-want) > bound+1e-16 {
				t.Fatal("independent cross-knot residual", crease)
			}
			a, b, c := got.Mesh.Vertices[tri[0]], got.Mesh.Vertices[tri[1]], got.Mesh.Vertices[tri[2]]
			area := b.Sub(a).Cross(c.Sub(a)).Z / 2
			if area <= 0 {
				t.Fatal("graph orientation")
			}
			volume += area * gap
		}
		if crossings == 0 {
			t.Fatal("fixture never crosses knot")
		}
		if math.Abs(volume-math.Pi/1024) > .0004 {
			t.Fatal("volume", volume)
		}
		type incidence struct{ fragment, sign int }
		edges := map[[2]int][]incidence{}
		for ti, tri := range got.Mesh.Triangles {
			for j, a := range tri {
				b := tri[(j+1)%3]
				sign := 1
				if a > b {
					a, b = b, a
					sign = -1
				}
				key := [2]int{a, b}
				edges[key] = append(edges[key], incidence{got.OriginalFragment[ti], sign})
			}
		}
		cut := 0
		for _, owners := range edges {
			if len(owners) == 2 && owners[0].fragment != owners[1].fragment {
				cut++
				if owners[0].sign == owners[1].sign {
					t.Fatal("cut incidence")
				}
			}
		}
		if cut != got.Subdivisions {
			t.Fatal("cut conformance", cut, got.Subdivisions)
		}
		report, err := collision.SelfIntersections(got.Mesh, collision.Options{MaxPairs: 1000000})
		if err != nil || !report.Complete || len(report.Contacts) != 0 {
			t.Fatal("exact display contacts", err)
		}

		low := o
		low.MaxSpanCells = 1
		if partial, err := sub.TessellateMultiSpanCurvedPartitionCertified(context.Background(), 0, line, low); err == nil || len(partial.Mesh.Triangles) != 0 {
			t.Fatal("span budget")
		}
		t.Log("crease", crease, "triangles", len(got.Mesh.Triangles), "crossings", crossings, "cells", got.SpanCellChecks, "pieces", got.SpanPieceChecks, "volume", volume)
	}
}
