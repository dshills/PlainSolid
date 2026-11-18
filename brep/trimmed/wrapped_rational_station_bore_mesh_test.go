package trimmed

import (
	"context"
	"math"
	"plainsolid/internal/slowtest"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestRationalStationBoreCertifiedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	surface := geom.SurfacePatchOptions{}
	outer, _, err := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{
		rationalStationBand(t, 0, 1, 3, 2, .75), rationalStationBand(t, 1, 3, 2, 3, .75),
	}, 1e-8, surface)
	if err != nil {
		t.Fatal(err)
	}
	inner, _, err := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{
		rationalStationBand(t, 0, 3, 1, 1, .75),
	}, 1e-8, surface)
	if err != nil {
		t.Fatal(err)
	}
	bore, _, err := CombineCoaxialWrappedRationalStationLofts(ctx, outer, inner, parametric.Difference, surface)
	if err != nil {
		t.Fatal(err)
	}
	o := CertifiedWrappedStationMeshOptions{Band: CertifiedWrappedFrustumMeshOptions{
		Patch: CertifiedPatchMeshOptions{Deflection: .3, MaxTriangleChecks: 50000}, MaxTriangles: 20000,
	}}
	cert, err := bore.TessellateWrappedRationalStationBoreCertified(ctx, o)
	if err != nil || !cert.Source.Complete || !cert.Collision.Complete || len(cert.Collision.Contacts) != 0 || cert.Mesh.ValidateSolid() != nil {
		t.Fatal("weighted bore must have a certified closed genus-one display", err)
	}
	vol, _ := cert.SignedDisplayVolume.Float64()
	want := rationalQuarterProfileArea(.75, 4096) * 16
	if math.Abs(vol-want) > .05*want || len(cert.StationVertices[0]) != 3 || len(cert.StationVertices[1]) != 2 {
		t.Fatal("preserve original station grids and approximate independent weighted volume", vol, want)
	}
	p, err := bore.PrepareWrappedRationalStationBoreDisplay(ctx, o, placement.ClosedDisplayOptions{})
	if err != nil || p.SquaredDistance().Cmp(cert.SourceImageSquaredDistance) != 0 || p.Caps() != cert.Source.Caps ||
		!slices.Equal(p.Walls()[0], cert.Source.OuterWalls) || !slices.Equal(p.Walls()[1], cert.Source.InnerWalls) {
		t.Fatal("prepared display must retain complete original bore face ownership", err)
	}
	ancestry := p.Ancestry()
	for _, use := range p.Uses() {
		if len(ancestry.EdgeVertices[use.Edge]) == 0 || !slices.Equal(use.Vertices, ancestry.EdgeVertices[use.Edge]) {
			t.Fatal("original bore use lost its exact shared rim samples", use)
		}
	}
	if m, err := bore.TessellateWrappedStationBoreCertified(ctx, o); err == nil || len(m.Mesh.Vertices) != 0 {
		t.Fatal("circular method should reject weighted source", err)
	}
}
