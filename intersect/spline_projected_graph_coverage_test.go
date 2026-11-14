package intersect

import (
	"context"
	"plainsolid/geom"
	"testing"
)

func TestProjectedCoverageUnsupportedRetainsGeneric(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: .25}, {X: .25, Y: 1}, {X: .75}, {X: .75, Y: 1}}, k, k, nil)
	b, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Y: .5, Z: -1}, {Y: .5, Z: 1}, {X: 1, Y: .5, Z: -1}, {X: 1, Y: .5, Z: 1}}, k, k, nil)
	pa, pb := Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}
	x, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	y, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	tube, e := geom.CertifySurfacePairTube(x, y, geom.SurfacePairTubeOptions{FreeAxis: 0})
	if e != nil || !tube.ProvenBranch {
		t.Fatal(e)
	}
	seeds := SplineSeedReport{Attempts: []SplineSeedCell{{Cell: geom.SurfaceSurfaceCell{DomainA: x.Domain(), DomainB: y.Domain()}, Tube: &tube}}}
	o := SplineTubeCoverageOptions{CommonXYProjected: &CommonXYProjectedCoverageOptions{}, Isolation: geom.SurfaceSurfaceIsolationOptions{ParameterTolerance: 1}}
	got, e := CertifySplineTubeCoverage(ctx, pa, pb, seeds, o)
	if e != nil || !got.CoversAllContacts {
		t.Fatal(e)
	}
	if got.CommonXYProjected == nil || len(got.CommonXYProjected.Projected) != 0 || len(got.CommonXYProjected.GenericFallback) != 1 || len(got.Covered) != 1 || got.CommonXYProjected.ExactOperations <= 0 {
		t.Fatal(got)
	}
	o.CommonXYProjected = &CommonXYProjectedCoverageOptions{MaxOperations: got.CommonXYProjected.ExactOperations}
	if r, e := CertifySplineTubeCoverage(ctx, pa, pb, seeds, o); e != nil || !r.CoversAllContacts {
		t.Fatal("exact fallback quota", e)
	}
	o.CommonXYProjected = &CommonXYProjectedCoverageOptions{MaxOperations: 1}
	if r, e := CertifySplineTubeCoverage(ctx, pa, pb, seeds, o); e == nil || r.CoversAllContacts || len(r.Unchecked)+len(r.Unresolved) == 0 {
		t.Fatal("fallback cannot reset quota", e)
	}
	o.CommonXYProjected = &CommonXYProjectedCoverageOptions{}
	duplicate := seeds
	duplicate.Attempts = append(append([]SplineSeedCell(nil), seeds.Attempts...), seeds.Attempts[0])
	r, e := CertifySplinePairBranches(ctx, pa, pb, duplicate, SplinePairTraceOptions{Coverage: o})
	if e == nil || r.Complete || len(r.Issues) == 0 {
		t.Fatal("duplicate topology bypass", e)
	}
}
