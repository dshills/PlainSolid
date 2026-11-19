//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestSolidCertifiedNativeEllipticalLensAdapter(t *testing.T) {
	slowtest.Skip(t)
	s := solidEllipticalLensFixture(t)
	o := SolidExactQuadraticMeshOptions{Shell: ExactQuadraticShellMeshOptions{Join: ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .02}}}}}}
	got, e := s.TessellateExactQuadraticShellCertified(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.OriginalEdges) != 4 || len(got.OriginalVertices) != 4 || got.ModelEdgeUsesCertified != 8 {
		t.Fatal("lost Solid topology")
	}
	if !got.ReparameterizedFaces[1] || got.ReparameterizedFaces[0] {
		t.Fatal("reversed face not handled")
	}
	for fi, mapping := range got.OriginalTrim {
		for j, original := range mapping {
			want := j
			if fi == 1 {
				want = len(mapping) - 1 - j
			}
			if original != want {
				t.Fatal("trim ancestry")
			}
		}
	}
	for j, id := range got.JunctionVertices {
		if got.Mesh.Vertices[id] != s.vertices[got.OriginalVertices[j]] {
			t.Fatal("corner topology changed")
		}
	}
	if !got.DisplayCollision.Complete || len(got.DisplayCollision.Contacts) != 0 {
		t.Fatal("embedding")
	}
	exact, _ := got.SignedDisplayVolume.Float64()
	want := math.Pi / 1024 // ellipse area pi/32, mean graph gap1/32.
	if math.Abs(exact-want) > want*.15 {
		t.Fatal("independent analytic lens volume", exact, want)
	}
	edges := map[[2]int]bool{}
	for _, tri := range got.Mesh.Triangles {
		for j, a := range tri {
			b := tri[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}] = true
		}
	}
	if len(got.Mesh.Vertices)-len(edges)+len(got.Mesh.Triangles) != 2 {
		t.Fatal("lens Euler characteristic")
	}
	report, e := collision.SelfIntersections(got.Mesh, collision.Options{MaxPairs: 1000000})
	if e != nil || !report.Complete || len(report.Contacts) != 0 {
		t.Fatal("independent display embedding", e)
	}
	t.Logf("triangles=%d volume=%g analytic=%g relative=%g BVH=%d", len(got.Mesh.Triangles), exact, want, (exact-want)/want, got.DisplayCollision.BroadPhasePairs)
	limited := o
	limited.Shell.Collision.MaxPairs = 1
	if r, e := s.TessellateExactQuadraticShellCertified(context.Background(), limited); !errors.Is(e, collision.ErrLimit) || len(r.Mesh.Triangles) != 0 {
		t.Fatal("partial adapter proof", e)
	}
}

func TestSolidNativeAdapterRejectsUnsupportedAndCancellation(t *testing.T) {
	s := solidEllipticalLensFixture(t)
	o := SolidExactQuadraticMeshOptions{Shell: ExactQuadraticShellMeshOptions{Join: ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .02}}}}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := s.TessellateExactQuadraticShellCertified(ctx, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := (Solid{}).TessellateExactQuadraticShellCertified(context.Background(), o); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal(e)
	}
	// A represented topology corner perturbed within the original tolerance is
	// not promoted into exact identity by this adapter.
	vertices := s.Vertices()
	vertices[0].Z = math.Nextafter(vertices[0].Z, math.Inf(1))
	altered, err := NewSolid(vertices, s.Edges(), s.Faces(), 1e-9)
	if err != nil {
		t.Fatal("tolerance-valid corner fixture", err)
	}
	if r, e := altered.TessellateExactQuadraticShellCertified(context.Background(), o); e == nil || len(r.Mesh.Triangles) != 0 {
		t.Fatal("approximate corner promoted")
	}
}

func TestSolidNativeAdapterReflectedPlacement(t *testing.T) {
	s := solidEllipticalLensFixture(t)
	moved, e := s.Transformed(geom.Affine{{-1, 0, 0, 2}, {0, 1, 0, 0}, {0, 0, 1, 1}})
	if e != nil {
		t.Fatal(e)
	}
	o := SolidExactQuadraticMeshOptions{Shell: ExactQuadraticShellMeshOptions{Join: ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .04}}}}}}
	got, e := moved.TessellateExactQuadraticShellCertified(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	if !got.ReparameterizedFaces[0] || got.ReparameterizedFaces[1] || got.SignedDisplayVolume.Sign() <= 0 {
		t.Fatal("physical orientation after reflection")
	}
	for j, id := range got.JunctionVertices {
		if got.Mesh.Vertices[id] != moved.vertices[j] {
			t.Fatal("placed original corner")
		}
	}
	if got.ModelEdgeUsesCertified != 8 || !got.DisplayCollision.Complete {
		t.Fatal("placed edge/embedding proof")
	}
}

func TestSolidNativeAdapterRejectsToleranceOnlyModelEdge(t *testing.T) {
	s := solidEllipticalLensFixture(t)
	edges := s.Edges()
	model, ok := edges[0].Curve.(surfaceTrimCurve)
	if !ok {
		t.Fatal("fixture edge descriptor")
	}
	surface, ok := model.surface.(*geom.BSplineSurface)
	if !ok {
		t.Fatal("fixture support")
	}
	data := surface.Data()
	delta := math.Ldexp(1, -32)
	for i := 0; i < data.CountU; i++ {
		for j := 0; j < data.CountV; j++ {
			// This perturbation vanishes at all four ellipse axis vertices but not
			// inside the quarter arc; its magnitude remains below constructor tolerance.
			data.Poles[i*data.CountV+j].Z += delta * (float64(i)/2 - .5) * (float64(j)/2 - .5)
		}
	}
	perturbed, e := geom.NewBSplineSurface(data.DegreeU, data.DegreeV, data.CountU, data.CountV, data.Poles, data.KnotsU, data.KnotsV, data.Weights)
	if e != nil {
		t.Fatal(e)
	}
	model.surface = perturbed
	edges[0].Curve = model
	accepted, e := NewSolid(s.Vertices(), edges, s.Faces(), 1e-9)
	if e != nil {
		t.Fatal("fixture must pass ordinary tolerance validation", e)
	}
	o := SolidExactQuadraticMeshOptions{Shell: ExactQuadraticShellMeshOptions{Join: ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .04}}}}}}
	r, e := accepted.TessellateExactQuadraticShellCertified(context.Background(), o)
	if e == nil || len(r.Mesh.Triangles) != 0 {
		t.Fatal("tolerance-only model edge promoted to exact proof")
	}
}

func solidNativeEllipseFixture(t *testing.T) Solid {
	t.Helper()
	s := solidEllipticalLensFixture(t)
	faces, edges := s.Faces(), s.Edges()
	native := make([]ExactQuadraticUV, len(edges))
	for i, edge := range edges {
		model := edge.Curve.(surfaceTrimCurve)
		source, ok := model.curve.(SplineUV)
		if !ok {
			t.Fatalf("fixture pcurve %T", model.curve)
		}
		poles := source.Curve.Poles()
		weights := []*big.Rat{big.NewRat(1, 1), big.NewRat(1, 3), big.NewRat(2, 9)}
		var h [3][3]*big.Rat
		for j, p := range poles {
			h[j] = [3]*big.Rat{new(big.Rat).Mul(ratInset(p.X), weights[j]), new(big.Rat).Mul(ratInset(p.Y), weights[j]), new(big.Rat).Set(weights[j])}
		}
		var e error
		native[i], e = NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
		// w0*w2=2*w1² proves the same exact quarter ellipse; only the
		// parameterization changes, using genuinely nonbinary normalized weights.
		model.curve = native[i]
		edges[i].Curve = model
	}
	for fi := range faces {
		for j, u := range faces[fi].Outer {
			faces[fi].Outer[j].Curve = native[u.Edge]
		}
	}
	exact, e := NewSolid(s.Vertices(), edges, faces, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	return exact
}

func TestSolidAdapterNativeNonbinaryEllipseWeights(t *testing.T) {
	exact := solidNativeEllipseFixture(t)
	o := SolidExactQuadraticMeshOptions{Shell: ExactQuadraticShellMeshOptions{Join: ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .04}}}}}}
	got, e := exact.TessellateExactQuadraticShellCertified(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	if got.Trims[0][0].Source.HomogeneousControls()[1][2].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("nonbinary native weight changed")
	}
	if got.ModelEdgeUsesCertified != 8 || !got.DisplayCollision.Complete || got.SignedDisplayVolume.Sign() <= 0 {
		t.Fatal("native Solid qualification")
	}
}
