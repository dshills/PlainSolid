package intersect

import (
	"context"
	"errors"
	"testing"

	"plainsolid/geom"
)

func TestCertifySplineTubeCoverage(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: .25}, {X: .25, Y: 1}, {X: .75}, {X: .75, Y: 1}}, k, k, nil)
	b, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Y: .5, Z: -1}, {Y: .5, Z: 1}, {X: 1, Y: .5, Z: -1}, {X: 1, Y: .5, Z: 1}}, k, k, nil)
	patch := func(s *geom.BSplineSurface) Patch { return Patch{s, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}} }
	pa, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	pb, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	var seeds SplineSeedReport
	for i := range 2 {
		da, db := pa.Domain(), pb.Domain()
		da.UMin, da.UMax = float64(i)/2, float64(i+1)/2
		p, e := pa.Trim(da)
		if e != nil {
			t.Fatal(e)
		}
		tube, e := geom.CertifySurfacePairTube(p, pb, geom.SurfacePairTubeOptions{})
		if e != nil || !tube.ProvenBranch {
			t.Fatal(tube, e)
		}
		seeds.Attempts = append(seeds.Attempts, SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: da, DomainB: db}, Tube: &tube})
	}
	opt := SplineTubeCoverageOptions{Isolation: geom.SurfaceSurfaceIsolationOptions{ParameterTolerance: 1},
		Cell: geom.SurfaceTubeUnionCoverageOptions{MaxCells: 16, MaxDepth: 3}}
	r, e := CertifySplineTubeCoverage(ctx, patch(a), patch(b), seeds, opt)
	if e != nil || !r.CoversAllContacts || len(r.Isolation.Candidates) != 1 || len(r.Covered) != 1 {
		t.Fatal(r, e)
	}
	seeds.Attempts = seeds.Attempts[:1]
	r, e = CertifySplineTubeCoverage(ctx, patch(a), patch(b), seeds, opt)
	if !errors.Is(e, ErrUnresolved) || r.CoversAllContacts || len(r.Unresolved) != 1 {
		t.Fatal(r, e)
	}
	// The identical input retains its explicit coincidence report. A report
	// supplied by the caller cannot turn it into a finite curve set.
	r, e = CertifySplineTubeCoverage(ctx, patch(a), patch(a), SplineSeedReport{}, opt)
	if !errors.Is(e, ErrUnresolved) || r.CoversAllContacts || len(r.Isolation.Coincident) != 1 {
		t.Fatal(r, e)
	}
}
