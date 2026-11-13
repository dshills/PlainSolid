package trimmed

import (
	"context"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedThreeCurvedFaceChain(t *testing.T) {
	corners := []geom.Vec3{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	var outer []SplineUV
	for i, p := range corners {
		c, e := geom.NewBSpline(1, []geom.Vec3{p, corners[(i+1)%4]}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		outer = append(outer, SplineUV{Curve: c, Range: UVInterval{Max: 1}})
	}
	var faces []QuadraticTrimStitchFace
	for strip := 0; strip < 3; strip++ {
		var poles []geom.Vec3
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				x := float64(strip)/2 + float64(i)/4
				y := float64(j) / 2
				z := x * y / 8
				if j == 2 {
					z++
				}
				poles = append(poles, geom.Vec3{X: x, Y: y, Z: z})
			}
		}
		k := []float64{0, 0, 0, 1, 1, 1}
		surface, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		faces = append(faces, QuadraticTrimStitchFace{Patch: patch, Outer: outer})
	}
	seams := []QuadraticTrimStitchSeam{{FaceA: 0, TrimA: 1, FaceB: 1, TrimB: 3}, {FaceA: 1, TrimA: 1, FaceB: 2, TrimB: 3}}
	options := QuadraticTrimFaceJoinOptions{Face: CurvedQuadraticTrimMeshOptions{Deflection: .02}}
	result, e := StitchQuadraticTrimCollectionCertified(context.Background(), faces, seams, options)
	if e != nil {
		t.Fatal(e)
	}
	n := result.Faces[0].Subdivisions
	if n < 2 || len(result.Mesh.Triangles) != 6*n*n || len(result.Mesh.Vertices) != 3*(n+1)*(n+1)-2*(n+1) {
		t.Fatal("independent strip grid counts", n, len(result.Mesh.Vertices), len(result.Mesh.Triangles))
	}
	for si, ids := range result.SeamVertices {
		if len(ids) != n+1 {
			t.Fatal("independent seam count", si, len(ids), n)
		}
		for j, id := range ids {
			p := result.Mesh.Vertices[id]
			if p.X != float64(si+1)/2 || math.Abs(p.Z-(p.Y*p.Y+p.X*p.Y/8)) > 1e-14 {
				t.Fatal("independent parabolic model seam", p)
			}
			if j > 0 && result.SeamParameters[si][j-1].Cmp(result.SeamParameters[si][j]) >= 0 {
				t.Fatal("unsorted seam ancestry")
			}
		}
	}
	seen := map[int]bool{}
	edges := map[[2]int]int{}
	for i, triangle := range result.Mesh.Triangles {
		seen[result.OriginalFace[i]] = true
		for j, a := range triangle {
			b := triangle[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}]++
		}
	}
	if len(seen) != 3 || len(result.Mesh.Vertices)-len(edges)+len(result.Mesh.Triangles) != 1 {
		t.Fatal("chain topology/ancestry")
	}
	for _, count := range edges {
		if count != 1 && count != 2 {
			t.Fatal("nonconforming chain")
		}
	}
	bound, _ := result.FillSquaredDistance.Float64()
	if bound > .02*.02 {
		t.Fatal("uncertified union distance")
	}
	// Reordering declared seams does not change assembled geometry or indices.
	again, e := StitchQuadraticTrimCollectionCertified(context.Background(), faces, []QuadraticTrimStitchSeam{seams[1], seams[0]}, options)
	if e != nil {
		t.Fatal(e)
	}
	for i, p := range result.Mesh.Vertices {
		if again.Mesh.Vertices[i] != p {
			t.Fatal("seam-order-dependent geometry")
		}
	}
	for i, tri := range result.Mesh.Triangles {
		if again.Mesh.Triangles[i] != tri {
			t.Fatal("seam-order-dependent topology")
		}
	}
	duplicate := append(append([]QuadraticTrimStitchSeam(nil), seams...), seams[0])
	bad, e := StitchQuadraticTrimCollectionCertified(context.Background(), faces, duplicate, options)
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("duplicate source seam accepted")
	}

	// Two valid seams meeting at a source vertex are not silently identified:
	// junction cyclic-link certification is outside this bounded collection API.
	var above []geom.Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			x, v := float64(i)/4, float64(j)/2
			y := 1 + v
			z := 1 + 2*v + x*y/8
			if j == 2 {
				z++
			}
			above = append(above, geom.Vec3{X: x, Y: y, Z: z})
		}
	}
	knots := []float64{0, 0, 0, 1, 1, 1}
	surface, e := geom.NewBSplineSurface(2, 2, 3, 3, above, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	junctionFaces := append([]QuadraticTrimStitchFace(nil), faces...)
	junctionFaces[2].Patch = patch
	bad, e = StitchQuadraticTrimCollectionCertified(context.Background(), junctionFaces, []QuadraticTrimStitchSeam{seams[0], {FaceA: 0, TrimA: 2, FaceB: 2, TrimB: 0}}, options)
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("unproved seam junction accepted")
	}

	junctionSeams := []QuadraticTrimStitchSeam{seams[0], {FaceA: 0, TrimA: 2, FaceB: 2, TrimB: 0}}
	declared := [][]QuadraticTrimSeamEndpoint{{{Seam: 0, End: 1}, {Seam: 1, End: 0}}}
	joined, e := StitchQuadraticTrimCollectionWithJunctionsCertified(context.Background(), junctionFaces, junctionSeams, declared, options)
	if e != nil {
		t.Fatal("explicit three-sector junction", e)
	}
	if len(joined.JunctionVertices) != 1 {
		t.Fatal("lost explicit junction")
	}
	junction := joined.JunctionVertices[0]
	if junction != joined.SeamVertices[0][len(joined.SeamVertices[0])-1] || junction != joined.SeamVertices[1][0] {
		t.Fatal("endpoint identities did not coalesce")
	}
	if joined.Mesh.Vertices[junction] != (geom.Vec3{X: .5, Y: 1, Z: 1.0625}) {
		t.Fatal("exact junction model point")
	}
	incident := map[int]bool{}
	link := map[int]int{}
	for i, tri := range joined.Mesh.Triangles {
		for j, id := range tri {
			if id == junction {
				incident[joined.OriginalFace[i]] = true
				link[tri[(j+1)%3]]++
				link[tri[(j+2)%3]]++
			}
		}
	}
	ends := 0
	for _, degree := range link {
		if degree == 1 {
			ends++
		} else if degree != 2 {
			t.Fatal("invalid junction fan degree")
		}
	}
	if len(incident) != 3 || ends != 2 {
		t.Fatal("not three sectors in one boundary fan", incident, ends)
	}
	for _, invalid := range [][][]QuadraticTrimSeamEndpoint{{{{Seam: 0, End: 0}, {Seam: 1, End: 0}}}, {{{Seam: 0, End: 2}, {Seam: 1, End: 0}}}, {{{Seam: 0, End: 1}, {Seam: 1, End: 0}}, {{Seam: 0, End: 1}, {Seam: 1, End: 0}}}} {
		bad, e = StitchQuadraticTrimCollectionWithJunctionsCertified(context.Background(), junctionFaces, junctionSeams, invalid, options)
		if e == nil || len(bad.Mesh.Vertices) != 0 {
			t.Fatal("conflicting junction evidence accepted")
		}
	}
	// Two independently stitched pairs share only a coincident endpoint. Their
	// disconnected fans cannot be made manifold by an explicit equality group.
	disconnected := []QuadraticTrimStitchFace{faces[0], faces[1], faces[0], faces[1]}
	separateSeams := []QuadraticTrimStitchSeam{{FaceA: 0, TrimA: 1, FaceB: 1, TrimB: 3}, {FaceA: 2, TrimA: 1, FaceB: 3, TrimB: 3}}
	bad, e = StitchQuadraticTrimCollectionWithJunctionsCertified(context.Background(), disconnected, separateSeams, [][]QuadraticTrimSeamEndpoint{{{Seam: 0, End: 0}, {Seam: 1, End: 0}}}, options)
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("disconnected coincident fan accepted")
	}
	options.Face.MaxTriangleChecks = 1
	bad, e = StitchQuadraticTrimCollectionCertified(context.Background(), faces, seams, options)
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("partial cumulative check budget")
	}
	t.Logf("three curved faces: %d triangles, %d vertices, %d samples per seam, bound %g", len(result.Mesh.Triangles), len(result.Mesh.Vertices), n+1, bound)
}
