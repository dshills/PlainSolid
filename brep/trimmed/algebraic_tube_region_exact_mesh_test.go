//go:build research

package trimmed

import (
	"context"
	"errors"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

// Qualify one annular component only: the nested inner disk touches this
// component along its inner rim, so this is not a disjoint collection proof.
func TestCertifiedExactOwnerAnnularRegion(t *testing.T) {
	slowtest.Skip(t)
	if testing.Short() {
		t.Skip("independent nested quartic source and annular display proof")
	}
	r := nestedTubeMaterialOwnerFixture(t, true)
	if r.section.partitionSource != nil || r.section.exactPartitionSource == nil {
		t.Fatal("fixture did not use exact native source")
	}
	for i, region := range r.Regions() {
		if len(region.Holes) != 1 {
			continue
		}
		faces := r.RegionFaces(i)
		if faces[0].Support != nil {
			t.Fatal("exact owner silently converted to float support")
		}
		if _, ok := faces[0].SourceSurface.(*geom.ExactBezierSurface); !ok {
			t.Fatal("exact source lost")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		failed, err := r.TessellateRegionClosedCertified(ctx, i, AlgebraicTubeRegionMeshOptions{})
		if !errors.Is(err, context.Canceled) || len(failed.Mesh.Vertices) != 0 {
			t.Fatal("cancellation must be atomic", err)
		}
		failed, err = r.TessellateRegionClosedCertified(context.Background(), i, AlgebraicTubeRegionMeshOptions{Surface: MultiSpanTubeLensMeshOptions{MaxSpanCells: -1}})
		if err == nil || len(failed.Mesh.Vertices) != 0 {
			t.Fatal("invalid limit must be atomic", err)
		}
		failed, err = r.TessellateRegionClosedCertified(context.Background(), i, AlgebraicTubeRegionMeshOptions{Footprint: AlgebraicTubeFootprintMeshOptions{Boundary: AlgebraicTubeRegionsBoundaryOptions{MaxLoops: 1}}})
		if !errors.Is(err, geom.ErrConvergence) || len(failed.Mesh.Vertices) != 0 {
			t.Fatal("insufficient loop quota must be atomic", err)
		}
		checkNestedTubeRegionShell(t, r, i)
		return
	}
	t.Fatal("missing annular region")
}
