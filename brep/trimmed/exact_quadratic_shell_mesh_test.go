//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func nativeClosedWarpedBoxFixture(t *testing.T) ([]ExactQuadraticShellFace, []QuadraticTrimStitchSeam, [][]QuadraticTrimSeamEndpoint) {
	t.Helper()
	const alpha = .25
	uvCorners := []geom.Vec3{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
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
	var faces []ExactQuadraticShellFace
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
		faces = append(faces, ExactQuadraticShellFace{Patch: p})
		for j, uv := range uvCorners {
			p := m.base(uv.X, uv.Y)
			corners[fi][j] = int(p.X) + 2*int(p.Y) + 4*int(p.Z)
		}
		for j, start := range uvCorners {
			end := uvCorners[(j+1)%4]
			ratio := big.NewRat(1, 3)
			if corners[fi][j] > corners[fi][(j+1)%4] {
				ratio = big.NewRat(2, 3)
			}
			h := [3][3]*big.Rat{{ratInset(start.X), ratInset(start.Y), big.NewRat(1, 1)}, {nil, nil, big.NewRat(1, 1)}, {ratInset(end.X), ratInset(end.Y), big.NewRat(1, 1)}}
			for k := 0; k < 2; k++ {
				h[1][k] = new(big.Rat).Add(h[0][k], new(big.Rat).Mul(ratio, new(big.Rat).Sub(h[2][k], h[0][k])))
			}
			trim, e := NewExactQuadraticUV(h)
			if e != nil {
				t.Fatal(e)
			}
			faces[fi].Outer = append(faces[fi].Outer, trim)
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

	return faces, seams, junctions
}
func TestEmbeddedNativeClosedWarpedShell(t *testing.T) {
	faces, seams, junctions := nativeClosedWarpedBoxFixture(t)
	o := ExactQuadraticShellMeshOptions{Join: ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .1}}}}}
	got, e := StitchEmbeddedExactQuadraticShellCertified(context.Background(), faces, seams, junctions, o)
	if e != nil {
		t.Fatal(e)
	}
	if got.Trims[0][0].Source.HomogeneousControls()[1][0].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("nonbinary native control was not retained")
	}
	if len(got.JunctionVertices) != 8 || len(got.SeamVertices) != 12 || len(got.Agreements) != 12 {
		t.Fatal("missing original shell ancestry")
	}
	if !got.DisplayCollision.Complete || len(got.DisplayCollision.Contacts) != 0 {
		t.Fatal("display embedding")
	}
	for _, agreement := range got.Agreements {
		if !agreement.Complete || !agreement.Equal {
			t.Fatal("whole seam not proved")
		}
	}
	edges := map[[2]int]int{}
	balances := map[[2]int]int{}
	for _, tri := range got.Mesh.Triangles {
		for j, a := range tri {
			b := tri[(j+1)%3]
			sign := 1
			if a > b {
				a, b = b, a
				sign = -1
			}
			key := [2]int{a, b}
			edges[key]++
			balances[key] += sign
		}
	}
	if len(got.Mesh.Vertices)-len(edges)+len(got.Mesh.Triangles) != 2 {
		t.Fatal("sphere Euler characteristic")
	}
	for key, n := range edges {
		if n != 2 || balances[key] != 0 {
			t.Fatal("closed opposite incidence")
		}
	}
	for fi, face := range got.Faces {
		if face.Subdivisions != 1 {
			t.Fatal("boundary refined")
		}
		for _, source := range face.OriginalFace {
			if source != fi {
				t.Fatal("face ancestry")
			}
		}
	}
	// This globally invertible polynomial shear has determinant1 and cube volume1.
	// Display volume is independently computed exactly from its represented
	// vertices; agreement with the source formula is fixture qualification.
	volume, _ := got.SignedDisplayVolume.Float64()
	if math.Abs(volume-1) > .005 || math.Abs(got.Mesh.SignedVolume()-volume) > 1e-12 {
		t.Fatal("independent volume", volume)
	}
	analyticArea := 4 + math.Sqrt(1.25) + math.Asinh(.5)/.5
	if math.Abs(got.Mesh.Area()-analyticArea) > .02 {
		t.Fatal("independent surface integral", got.Mesh.Area(), analyticArea)
	}
	independent, e := collision.SelfIntersections(got.Mesh, collision.Options{MaxPairs: 1000000})
	if e != nil || !independent.Complete || len(independent.Contacts) != 0 {
		t.Fatal("independent sweep embedding", e)
	}
	t.Logf("triangles=%d vertices=%d exact-display-volume=%g source-volume=1 area=%g BVH=%d", len(got.Mesh.Triangles), len(got.Mesh.Vertices), volume, got.Mesh.Area(), got.DisplayCollision.BroadPhasePairs)
	if r, e := StitchEmbeddedExactQuadraticShellCertified(context.Background(), faces, seams, nil, o); e == nil || len(r.Mesh.Triangles) != 0 {
		t.Fatal("undeclared junctions accepted")
	}
	limited := o
	limited.Collision.MaxPairs = 1
	if r, e := StitchEmbeddedExactQuadraticShellCertified(context.Background(), faces, seams, junctions, limited); !errors.Is(e, collision.ErrLimit) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("partial collision proof", e)
	}
}
func TestEmbeddedNativeShellRejectsIncompleteTopology(t *testing.T) {
	faces, seams, junctions := nativeClosedWarpedBoxFixture(t)
	o := ExactQuadraticShellMeshOptions{Join: ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .1}}}}}
	if r, e := StitchEmbeddedExactQuadraticShellCertified(context.Background(), faces, seams[:len(seams)-1], junctions, o); !errors.Is(e, geom.ErrInvalid) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("missing seam accepted", e)
	}
	badCorners := make([][]QuadraticTrimSeamEndpoint, len(junctions))
	for i, g := range junctions {
		badCorners[i] = append([]QuadraticTrimSeamEndpoint(nil), g...)
	}
	badCorners[0][0], badCorners[7][0] = badCorners[7][0], badCorners[0][0]
	if r, e := StitchEmbeddedExactQuadraticShellCertified(context.Background(), faces, seams, badCorners, o); e == nil || len(r.Mesh.Triangles) != 0 {
		t.Fatal("incorrect exact corner identities accepted")
	}
	o.MaxTriangles = 1
	if r, e := StitchEmbeddedExactQuadraticShellCertified(context.Background(), faces, seams, junctions, o); !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("partial aggregate proof", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := StitchEmbeddedExactQuadraticShellCertified(ctx, faces, seams, junctions, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
