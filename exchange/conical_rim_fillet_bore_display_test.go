package exchange_test

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestImportedConicalRimFilletBoreCertifiedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	const fixture = "/testdata/occt-conical-double-rim-eccentric-bore-exact."
	read := func(path string) []byte {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	stepImport, err := step.ReadConicalRimFilletBore(ctx, bytes.NewReader(read("step"+fixture+"step")), step.Limits{}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	igesImport, err := iges.ReadConicalRimFilletBore(ctx, bytes.NewReader(read("iges"+fixture+"igs")), iges.Limits{}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	native := func(s trimmed.Solid) []byte {
		t.Helper()
		var b bytes.Buffer
		if err := trimmed.WriteJSON(&b, s); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	expected := native(stepImport.Solid)
	if !bytes.Equal(native(igesImport.Solid), expected) {
		t.Fatal("independently authored STEP and IGES reconstructed different native geometry")
	}
	const limit = .2
	o := trimmed.CertifiedWrappedFrustumMeshOptions{Patch: trimmed.CertifiedPatchMeshOptions{
		Deflection: limit, MaxSubdivisions: 16, MaxTriangleChecks: 30000,
	}, MaxTriangles: 20000}
	for _, input := range []struct {
		name string
		mesh func(context.Context, trimmed.CertifiedWrappedFrustumMeshOptions) (trimmed.CertifiedFilletedFrustumBoreMesh, error)
	}{
		{"STEP", stepImport.TessellateCertified}, {"IGES", igesImport.TessellateCertified},
	} {
		t.Run(input.name, func(t *testing.T) {
			display, err := input.mesh(ctx, o)
			if err != nil {
				t.Fatal(err)
			}
			if err := display.Mesh.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			if !display.Collision.Complete || len(display.Collision.Contacts) != 0 ||
				display.SignedDisplayVolume.Sign() <= 0 || display.SquaredDeflection.Cmp(big.NewRat(1, 25)) > 0 ||
				len(display.OriginalFace) != len(display.Mesh.Triangles) ||
				!bytes.Equal(native(display.Source), expected) {
				t.Fatal("display certificate differs from imported native material")
			}
			var observed [6]bool
			for _, id := range display.OriginalFace {
				if id < 0 || id >= len(observed) {
					t.Fatalf("mesh triangle has invalid source face %d", id)
				}
				observed[id] = true
			}
			for id, found := range observed {
				if !found {
					t.Fatalf("mesh omitted original face %d", id)
				}
			}
			t.Logf("%d triangles, %d exact patch checks, %d predicates", len(display.Mesh.Triangles), display.TriangleChecks, display.Predicates)
		})
	}
}
