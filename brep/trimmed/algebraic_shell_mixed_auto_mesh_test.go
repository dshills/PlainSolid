package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestMixedOriginalAutomaticCompleteMesh(t *testing.T) {
	ctx := context.Background()
	a, b, proposals, sectionOptions := mixedWallFixture(t)
	section, err := SectionSplineShellsMixedAlgebraic(ctx, a, b, proposals, sectionOptions)
	if err != nil {
		t.Fatal(err)
	}
	booleanOptions := AlgebraicShellBooleanOptions{}
	booleanOptions.Imprint.Exact.MaxOperations = 128000000
	bodies, err := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection}, booleanOptions)
	if err != nil {
		t.Fatal(err)
	}
	body := bodies[0]
	o := MixedIntersectionMeshOptions{
		Boundary: AlgebraicShellBoundaryEmbeddingOptions{Boundary: AlgebraicShellBoundaryOptions{
			Deflection: .02, MaxOperations: 128000000, AutoSectionProjection: true, PlanarSectionFallback: true}},
		Roof: MixedRoofCollarOptions{Roof: MixedRoofInteriorOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .2}}, Width: .04},
	}
	got, err := body.TessellateIntersectionMeshCertified(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	m := got.Mesh()
	if got.SourceMaterial() != body || len(m.Vertices) != 1004 || len(m.Triangles) != 2004 ||
		got.SourceImageSquaredDistance().Cmp(big.NewRat(1, 100)) >= 0 {
		t.Fatal("automatically inferred roles must preserve the exact source and close all faces", len(m.Vertices), len(m.Triangles))
	}
	for _, v := range got.TriangleFaces() {
		if v < 0 || v >= len(body.boundary.faces) {
			t.Fatal("native material face identity", v)
		}
	}
	o.SupportOperations = 1
	if got, err := body.TessellateIntersectionMeshCertified(ctx, o); got != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("insufficient support discovery must fail atomically", got, err)
	}
}
