//go:build research

package trimmed

import (
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedClosedCurvedShearedBox(t *testing.T) {
	const alpha = .25
	uvCorners := []geom.Vec3{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	var outer []SplineUV
	for i, p := range uvCorners {
		c, e := geom.NewBSpline(1, []geom.Vec3{p, uvCorners[(i+1)%4]}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		outer = append(outer, SplineUV{Curve: c, Range: UVInterval{Max: 1}})
	}
	// F(x,y,z)=(x,y,z+alpha*y²) is globally invertible with determinant1.
	// The six oriented cube charts are independently authored below.
	mappings := []struct {
		base  func(float64, float64) geom.Vec3
		yAxis int
	}{
		{func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: 1 - v} }, 1},
		{func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v, Z: 1} }, 1},
		{func(u, v float64) geom.Vec3 { return geom.Vec3{Y: 1 - u, Z: v} }, 0},
		{func(u, v float64) geom.Vec3 { return geom.Vec3{X: 1, Y: u, Z: v} }, 0},
		{func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Z: v} }, -1},
		{func(u, v float64) geom.Vec3 { return geom.Vec3{X: 1 - u, Y: 1, Z: v} }, -1},
	}
	var faces []QuadraticTrimStitchFace
	corners := make([][4]int, len(mappings))
	for fi, m := range mappings {
		var poles []geom.Vec3
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				p := m.base(float64(i)/2, float64(j)/2)
				p.Z += alpha * p.Y * p.Y
				if m.yAxis == 0 && i == 1 || m.yAxis == 1 && j == 1 {
					p.Z -= alpha / 4
				}
				poles = append(poles, p)
			}
		}
		knots := []float64{0, 0, 0, 1, 1, 1}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		faces = append(faces, QuadraticTrimStitchFace{Patch: p, Outer: outer})
		for j, uv := range uvCorners {
			p := m.base(uv.X, uv.Y)
			corners[fi][j] = int(p.X) + 2*int(p.Y) + 4*int(p.Z)
		}
	}
	type originalUse struct{ face, trim, start, end int }
	pending := map[[2]int]originalUse{}
	var seams []QuadraticTrimStitchSeam
	var ends [][2]int
	for fi, c := range corners {
		for j, a := range c {
			b := c[(j+1)%4]
			key := [2]int{a, b}
			if a > b {
				key = [2]int{b, a}
			}
			if first, ok := pending[key]; ok {
				if first.start != b || first.end != a {
					t.Fatal("fixture orientation")
				}
				seams = append(seams, QuadraticTrimStitchSeam{FaceA: first.face, TrimA: first.trim, FaceB: fi, TrimB: j})
				ends = append(ends, [2]int{first.start, first.end})
				delete(pending, key)
			} else {
				pending[key] = originalUse{fi, j, a, b}
			}
		}
	}
	if len(pending) != 0 || len(seams) != 12 {
		t.Fatal("cube edge ownership fixture")
	}
	junctions := make([][]QuadraticTrimSeamEndpoint, 8)
	for si, vertices := range ends {
		for endpoint, corner := range vertices {
			junctions[corner] = append(junctions[corner], QuadraticTrimSeamEndpoint{Seam: si, End: endpoint})
		}
	}
	for _, group := range junctions {
		if len(group) != 3 {
			t.Fatal("cube corner degree")
		}
	}
	options := QuadraticTrimFaceJoinOptions{Face: CurvedQuadraticTrimMeshOptions{Deflection: .01}}
	result, e := StitchClosedQuadraticShellCertified(context.Background(), faces, seams, junctions, options)
	if e != nil {
		t.Fatal(e)
	}
	n := result.Faces[0].Subdivisions
	if len(result.Mesh.Triangles) != 12*n*n || len(result.Mesh.Vertices) != 6*n*n+2 || len(result.JunctionVertices) != 8 {
		t.Fatal("independent closed cube-grid counts", n, len(result.Mesh.Vertices), len(result.Mesh.Triangles))
	}
	if e = result.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if math.Abs(result.Mesh.SignedVolume()-1) > 1e-12 {
		t.Fatal("unit determinant shear volume", result.Mesh.SignedVolume())
	}
	exactArea := 4 + math.Sqrt(1+4*alpha*alpha) + math.Asinh(2*alpha)/(2*alpha)
	if result.Mesh.Area() > exactArea+1e-12 || exactArea-result.Mesh.Area() > .002 {
		t.Fatal("independent surface integral", result.Mesh.Area(), exactArea)
	}
	// This separate exact triangle predicate qualifies the DISPLAY embedding;
	// source embedding follows only for this independently invertible fixture.
	report, e := collision.SelfIntersections(result.Mesh, collision.Options{MaxPairs: 1000000, MaxContacts: 1})
	if e != nil || !report.Complete || len(report.Contacts) != 0 {
		t.Fatal("display self-intersection", report, e)
	}
	embedding, e := result.CheckDisplayEmbedding(context.Background(), collision.Options{MaxPairs: 1000000, MaxContacts: 1})
	if e != nil || embedding.Status != DisplayEmbeddingCertified {
		t.Fatal("opt-in exact closed display embedding", embedding, e)
	}
	bad, e := StitchClosedQuadraticShellCertified(context.Background(), faces, seams[:len(seams)-1], junctions, options)
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("incomplete original edge ownership accepted")
	}
	bad, e = StitchClosedQuadraticShellCertified(context.Background(), faces, seams, nil, options)
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("undeclared closed corner identity accepted")
	}
	options.Face.MaxTriangleChecks = 1
	bad, e = StitchClosedQuadraticShellCertified(context.Background(), faces, seams, junctions, options)
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("partial closed shell on budget failure")
	}
	t.Logf("closed curved shell: %d triangles, %d vertices, volume %.15g, area %.15g (analytic %.15g)", len(result.Mesh.Triangles), len(result.Mesh.Vertices), result.Mesh.SignedVolume(), result.Mesh.Area(), exactArea)
}
