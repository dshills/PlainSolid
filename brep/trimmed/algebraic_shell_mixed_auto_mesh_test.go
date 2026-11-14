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
	a, b, _, sectionOptions := mixedWallFixture(t)
	section, err := DiscoverSplineShellsMixedAlgebraic(ctx, a, b, sectionOptions, MixedRationalIsoDiscoveryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	booleanOptions := AlgebraicShellBooleanOptions{}
	booleanOptions.Imprint.Exact.MaxOperations = 128000000
	bodies, err := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection, parametric.Difference, parametric.Union}, booleanOptions)
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
	diff := bodies[1]
	if got, err := diff.TessellateIntersectionMeshCertified(ctx, o); got != nil || !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatal("intersection entry must reject a difference before tessellating", got, err)
	}
	result, err := diff.TessellateDifferenceMeshCertified(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	m = result.Mesh()
	if result.SourceMaterial() != diff || len(m.Vertices) != 1013 || len(m.Triangles) != 2018 ||
		result.SourceImageSquaredDistance().Cmp(big.NewRat(1, 100)) >= 0 {
		t.Fatal("automatic difference lost original source or signed complement coverage", len(m.Vertices), len(m.Triangles))
	}
	if got, err := diff.TessellateUnionMeshCertified(ctx, o); got != nil || !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatal("union entry must reject a difference before tessellating", got, err)
	}
	union := bodies[2]
	result, err = union.TessellateUnionMeshCertified(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	m = result.Mesh()
	if result.SourceMaterial() != union || len(m.Vertices) != 1040 || len(m.Triangles) != 2076 ||
		result.SourceImageSquaredDistance().Cmp(big.NewRat(21, 1000)) >= 0 {
		t.Fatal("automatic union lost original source or complementary roof coverage", len(m.Vertices), len(m.Triangles))
	}
	byFace := make(map[int]int)
	for _, id := range result.TriangleFaces() {
		byFace[id]++
	}
	if len(byFace) != len(union.Faces()) || byFace[12] != 606 {
		t.Fatal("the automatic fill must cover all original fragments including the holed roof", byFace)
	}
	o.SupportOperations = 1
	if got, err := body.TessellateIntersectionMeshCertified(ctx, o); got != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("insufficient support discovery must fail atomically", got, err)
	}
}

func TestMixedAutomaticTiltedSupportProof(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	// Degree elevation of Z = 3X/2 + Y/2 + 1/8 with
	// X=U, Y=V. Deliberately perturb the interior pole only;
	// the nonplanar case has identical corners and boundary.
	for _, bowed := range []bool{false, true} {
		var controls [][4]*big.Rat
		for i := range 3 {
			for j := range 3 {
				h := q(int64(6*i+2*j+1), 8)
				if bowed && i == 1 && j == 1 {
					h.Add(h, q(1, 1<<18))
				}
				controls = append(controls, [4]*big.Rat{q(int64(i), 2), q(int64(j), 2), h, q(1, 1)})
			}
		}
		patch, err := geom.NewExactHomogeneousBezierSurface(ctx, 2, 2, controls, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		left := 20000
		proposal, planar, err := mixedExactPlaneProposal(ctx, patch, 7, &left)
		if err != nil || planar == bowed {
			t.Fatal("original homogeneous interior determines planarity", bowed, planar, err)
		}
		if planar {
			if proposal.Face != 7 || proposal.ProjectionAxes != [2]int{1, 2} {
				t.Fatal("use the exact largest normal component to select axes", proposal)
			}
			proof, err := proveMixedPhysicalPlane(ctx, patch, proposal, 100000)
			if err != nil || proof.orientation == 0 {
				t.Fatal("the tilted projection must be injective", proof, err)
			}
		}
	}
}
