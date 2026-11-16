package trimmed_test

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"os"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestCertifiedTranslatedConicalFilletBoreSTEPFacetNormals(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile("../../exchange/step/testdata/occt-conical-double-rim-eccentric-bore-exact.step")
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := step.ReadConicalRimFilletBore(ctx, bytes.NewReader(raw), step.Limits{}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Inputs from the pinned OCCT author, including the translated cone
	// and a bore displaced on both axes relative to its center.
	stock, bore := translatedConicalFilletBoreStock(t, ctx)
	o := trimmed.CertifiedWrappedFrustumMeshOptions{
		Patch: trimmed.CertifiedPatchMeshOptions{Deflection: .5, MaxSubdivisions: 16, MaxTriangleChecks: 100000},
	}
	loose, err := trimmed.TessellateFilletedFrustumThroughBoreNormalsCertified(ctx, stock, 15./16, 15./8, bore, 1, .2, o)
	if err != nil {
		t.Fatal(err)
	}
	tight, err := trimmed.TessellateFilletedFrustumThroughBoreNormalsCertified(ctx, stock, 15./16, 15./8, bore, 1, .1, o)
	if err != nil {
		t.Fatal(err)
	}
	// The bridge can use the .1 facet cone for .2 pairwise source turn.
	request := new(big.Rat).SetFloat64(.1)
	request.Mul(request, request)
	if tight.SquaredNormalTangent == nil || tight.SquaredNormalTangent.Sign() <= 0 || tight.SquaredNormalTangent.Cmp(request) > 0 ||
		loose.SquaredNormalTangent == nil || tight.SquaredNormalTangent.Cmp(loose.SquaredNormalTangent) >= 0 ||
		len(tight.Mesh.Triangles) <= len(loose.Mesh.Triangles) {
		t.Fatalf("tightening the angle did not refine and improve the certificate: loose %v, tight %v", loose.SquaredNormalTangent, tight.SquaredNormalTangent)
	}
	if tight.SquaredDeflection == nil || tight.SquaredDeflection.Cmp(big.NewRat(1, 4)) > 0 ||
		!tight.Collision.Complete || len(tight.Collision.Contacts) != 0 || tight.Mesh.ValidateSolid() != nil ||
		tight.Source.Topology().EulerCharacteristic != 0 || len(tight.OriginalFace) != len(tight.Mesh.Triangles) {
		t.Fatal("lost bounded distance, closed annular incidence, original ownership, or contact proof")
	}
	for end, rims := range tight.Rims {
		for kind, row := range rims {
			if len(row) <= len(loose.Rims[end][kind]) {
				t.Fatalf("small angle did not refine rim %d of cap %d", kind, end)
			}
		}
	}
	// The mesh retains the native source rebuilt from the authored STEP
	// supports, including their original parameter intervals.
	var wanted, got bytes.Buffer
	if err := trimmed.WriteJSON(&wanted, pinned.Solid); err != nil {
		t.Fatal(err)
	}
	if err := trimmed.WriteJSON(&got, tight.Source); err != nil || !bytes.Equal(wanted.Bytes(), got.Bytes()) {
		t.Fatal("mesh source differs from the strict, pinned STEP native source", err)
	}
	faces := tight.History.Fillets
	seen := make(map[int]int)
	for i, tri := range tight.Mesh.Triangles {
		a, b, c := tight.Mesh.Vertices[tri[0]], tight.Mesh.Vertices[tri[1]], tight.Mesh.Vertices[tri[2]]
		n := b.Sub(a).Cross(c.Sub(a))
		owner := tight.OriginalFace[i]
		seen[owner]++
		switch owner {
		case faces.Caps[0]:
			if n.Z >= 0 {
				t.Fatal("lower cap must face down")
			}
		case faces.Caps[1]:
			if n.Z <= 0 {
				t.Fatal("upper cap must face up")
			}
		case tight.History.BoreFace:
			if n.X*(a.X-bore.X)+n.Y*(a.Y-bore.Y) >= 0 {
				t.Fatal("eccentric bore material normal must face the translated cutter axis")
			}
		}
	}
	if len(seen) != 6 {
		t.Fatalf("missing pinned source face in angular display: %v", seen)
	}
	mass, err := pinned.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-2282.4014009763955) > 1e-7 {
		t.Fatal("the pinned native solid must retain the independently authored volume", mass, err)
	}
	coarseVolume, _ := loose.SignedDisplayVolume.Float64()
	volume, _ := tight.SignedDisplayVolume.Float64()
	if volume <= 0 || math.Abs(volume-mass.Volume) > .01*mass.Volume ||
		math.Abs(volume-mass.Volume) >= math.Abs(coarseVolume-mass.Volume) {
		t.Fatalf("unexpected signed mesh volumes: loose %g, tight %g, source %g", coarseVolume, volume, mass.Volume)
	}
	t.Logf("loose: triangles=%d tangent²=%s distance²=%s signed volume=%.12f", len(loose.Mesh.Triangles), loose.SquaredNormalTangent.FloatString(12), loose.SquaredDeflection.FloatString(12), coarseVolume)
	t.Logf("tight: triangles=%d tangent²=%s distance²=%s signed volume=%.12f source volume=%.12f candidates=%d unwanted contacts=%d checks=%d predicates=%d",
		len(tight.Mesh.Triangles), tight.SquaredNormalTangent.FloatString(12), tight.SquaredDeflection.FloatString(12), volume, mass.Volume,
		tight.Collision.BroadPhasePairs, len(tight.Collision.Contacts), tight.TriangleChecks, tight.Predicates)
}

func TestCertifiedTranslatedConicalFilletBoreAngularBudgetRefusesDistanceOnly(t *testing.T) {
	ctx := context.Background()
	stock, bore := translatedConicalFilletBoreStock(t, ctx)
	o := trimmed.CertifiedWrappedFrustumMeshOptions{Patch: trimmed.CertifiedPatchMeshOptions{Deflection: 100, MaxSubdivisions: 1}}
	coarse, err := trimmed.TessellateFilletedFrustumThroughBoreCertified(ctx, stock, 15./16, 15./8, bore, 1, o)
	if err != nil || len(coarse.Mesh.Triangles) == 0 || coarse.SquaredNormalTangent != nil {
		t.Fatal("coarse budget should admit the distance-only request", err)
	}
	failed, err := trimmed.TessellateFilletedFrustumThroughBoreNormalsCertified(ctx, stock, 15./16, 15./8, bore, 1, .1, o)
	if !errors.Is(err, geom.ErrConvergence) || failed.SquaredNormalTangent != nil ||
		failed.SquaredDeflection != nil || len(failed.Mesh.Triangles) != 0 || failed.Source.Topology().Faces != 0 {
		t.Fatal("coarse budget must refuse the requested normal cone atomically", err)
	}
}

func translatedConicalFilletBoreStock(t *testing.T, ctx context.Context) (trimmed.Solid, geom.Vec2) {
	t.Helper()
	o := geom.SurfacePatchOptions{}
	w, err := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{Y: 2}, 8, 6, o)
	if err != nil {
		t.Fatal(err)
	}
	w, _, err = geom.DraftWrappedFrustum(ctx, w, 0, 8, o)
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := trimmed.NewWrappedFrustum(ctx, w, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	return s, geom.Vec2{X: 1.5, Y: 4}
}
