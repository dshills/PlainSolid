package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestMappedSplineChainRectangularSolid(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	boundary, closing, planar, seams := mappedChainClosedFixture(t)
	shell, err := CertifyMappedSplineChainShell(ctx, boundary, closing, [2]bool{false, true}, planar, seams, MappedChainShellProofOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out, err := shell.BuildRectangularSolid(ctx, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Solid.faces) != 7 || len(out.Solid.edges) != 17 || len(out.Solid.vertices) != 12 ||
		len(out.ChainEdges) != 3 || len(out.ClosingEdges) != 14 {
		t.Fatal("incorrect topology after introducing original shared stations")
	}
	checkOwners := func(s Solid) {
		t.Helper()
		for j, edge := range out.ChainEdges {
			for owner := range boundary.sources {
				use := s.faces[owner].Outer[j]
				trim, ok := use.Curve.(MappedSplineUV)
				if use.Edge != edge || use.Reversed || !ok ||
					!sameAuthoredBezierSpline(trim.Source().Curve, boundary.sources[owner].parts[0].Source().Curve) {
					t.Fatal("original mapped coedge", owner, j, use)
				}
				for k := range 2 {
					got, err := trim.AuthoredParameter(big.NewRat(int64(k), 1))
					if err != nil {
						t.Fatal(err)
					}
					want, err := boundary.sources[owner].AuthoredParameter(boundary.cuts[j+k])
					if err != nil || want.Cmp(got) != 0 {
						t.Fatal("original raw station", owner, j, k, got, want, err)
					}
				}
			}
		}
		basis, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if err != nil || !basis.Complete || len(basis.Uses) != 34 {
			t.Fatal("native continuous source coedge agreement", basis, err)
		}
	}
	checkOwners(out.Solid)
	var record bytes.Buffer
	if err := WriteJSON(&record, out.Solid); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadJSON(&record, JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	checkOwners(loaded)
	t.Logf("native topology vertices=%d edges=%d faces=%d source cells=%d",
		len(loaded.vertices), len(loaded.edges), len(loaded.faces), len(out.ChainEdges))
	mass, err := loaded.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
	if err != nil || math.Abs(mass.Volume-57.0/64) > 1e-7 {
		t.Fatal("independent analytic volume", mass.Volume, err)
	}
	faces := make([]SolidMixedFaceMeshOptions, len(loaded.faces))
	for i := range faces {
		faces[i].Face.Face = MixedExactCurvedMeshOptions{
			BoundaryModelBudget: .005,
			Interior:            CurvedQuadraticTrimMeshOptions{Deflection: .04},
		}
	}
	display, err := loaded.TessellateMixedClosedCertified(ctx, StoredMixedClosedMeshOptions{Faces: faces})
	if err != nil || !display.DisplayCollision.Complete || len(display.DisplayCollision.Contacts) != 0 {
		t.Fatal("ordinary stored-solid closed display after native reload", err)
	}
	t.Logf("volume=%.12g native closed triangles=%d", mass.Volume, len(display.Mesh.Triangles))

	for _, bad := range []struct {
		tolerance float64
		options   geom.SurfacePatchOptions
	}{
		{0, geom.SurfacePatchOptions{}},
		{1e-9, geom.SurfacePatchOptions{MaxOperations: 1}},
	} {
		got, err := shell.BuildRectangularSolid(ctx, bad.tolerance, bad.options)
		if err == nil || len(got.Solid.faces) != 0 || len(got.ChainEdges) != 0 {
			t.Fatal("invalid or incomplete source admission result")
		}
	}
}
